package app

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// ErrMotivoRequerido indica que se intentó otorgar días sin justificación.
var ErrMotivoRequerido = errors.New("el motivo es obligatorio")

// OtorgarManual carga un saldo especial a un colaborador. El motivo es
// obligatorio: un movimiento del ledger sin explicación es exactamente el
// problema que este sistema viene a resolver.
func (s *Servicio) OtorgarManual(
	ctx context.Context,
	colaboradorID, tipoID uuid.UUID,
	dias decimal.Decimal,
	motivo string,
	actorID uuid.UUID,
) error {
	if motivo == "" {
		return ErrMotivoRequerido
	}
	if !dias.IsPositive() {
		return errors.New("los días otorgados deben ser positivos")
	}

	hoy := s.Hoy()

	colaborador, err := store.ColaboradorPorID(ctx, s.Pool, s.EmpresaID, colaboradorID)
	if err != nil {
		return err
	}
	tipos, err := store.TiposPorID(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return err
	}
	tipo, existe := tipos[tipoID]
	if !existe {
		return errors.New("tipo de vacación desconocido")
	}

	// El período de una carga manual es el año en curso desde la fecha de carga.
	resultado := domain.ResultadoDevengo{
		Dias:         dias,
		DiasBase:     dias,
		PeriodoDesde: hoy,
		PeriodoHasta: hoy.AddDate(1, 0, -1),
		DevengadoEl:  hoy,
		Detalle:      motivo,
	}

	_, err = s.otorgar(ctx, colaborador, tipo, resultado, domain.OrigenManual, actorID, motivo)
	return err
}

// CrearTipo registra un tipo de vacación nuevo. Esta llamada es la respuesta
// completa al Requisito 7: agregar "días por rendimiento" es un INSERT.
func (s *Servicio) CrearTipo(ctx context.Context, t domain.TipoDeVacacion) (domain.TipoDeVacacion, error) {
	t.EmpresaID = s.EmpresaID
	if t.VigenteDesde.IsZero() {
		t.VigenteDesde = s.Hoy()
	}
	if err := validarTipo(t); err != nil {
		return domain.TipoDeVacacion{}, err
	}

	id, err := store.CrearTipo(ctx, s.Pool, t)
	if err != nil {
		return domain.TipoDeVacacion{}, err
	}
	t.ID = id
	return t, nil
}

// ActualizarTipo modifica un tipo existente.
func (s *Servicio) ActualizarTipo(ctx context.Context, t domain.TipoDeVacacion) error {
	t.EmpresaID = s.EmpresaID
	if err := validarTipo(t); err != nil {
		return err
	}
	return store.ActualizarTipo(ctx, s.Pool, t)
}

// ListarTipos devuelve la configuración de tipos de la empresa.
func (s *Servicio) ListarTipos(ctx context.Context) ([]domain.TipoDeVacacion, error) {
	return store.ListarTipos(ctx, s.Pool, s.EmpresaID)
}

// validarTipo acota la libertad de configuración. Sin esto, RRHH puede crear un
// tipo que devenga por año calendario sin días base, o uno que vence en n
// períodos sin decir cuántos.
func validarTipo(t domain.TipoDeVacacion) error {
	if t.Codigo == "" {
		return errors.New("el código es obligatorio")
	}
	if t.Nombre == "" {
		return errors.New("el nombre es obligatorio")
	}

	// Los parámetros llegan como jsonb editable desde la pantalla de RRHH.
	// Validarlos acá es lo que impide que un "15,5" con coma se persista y
	// después reviente en cada consulta de saldo.
	if err := t.Parametros.Validar(); err != nil {
		return err
	}

	switch t.PoliticaDevengo {
	case domain.DevengoAniversarioLegal, domain.DevengoAnioCalendario:
		if !t.Parametros.DiasBaseDecimal().IsPositive() {
			return errors.New("un tipo con devengo automático necesita días base positivos")
		}
	case domain.DevengoManual:
		// No necesita días base: los define RRHH en cada otorgamiento.
	default:
		return errors.New("política de devengo desconocida")
	}

	switch t.PoliticaVencimiento {
	case domain.VencimientoNPeriodos:
		if t.Parametros.NPeriodos <= 0 {
			return errors.New("la política n_periodos necesita un número de períodos positivo")
		}
	case domain.VencimientoDiasFijos:
		if t.Parametros.DiasFijos <= 0 {
			return errors.New("la política dias_fijos necesita un número de días positivo")
		}
	case domain.VencimientoNoVence, domain.VencimientoFinDeAnio:
		// Sin parámetros.
	default:
		return errors.New("política de vencimiento desconocida")
	}

	if t.Parametros.ProgresivoActivo {
		if t.PoliticaDevengo != domain.DevengoAniversarioLegal {
			return errors.New("el progresivo solo aplica a la política aniversario_legal")
		}
		if t.Parametros.ProgresivoCadenciaAnios <= 0 {
			return errors.New("el progresivo necesita una cadencia en años positiva")
		}
	}

	return nil
}
