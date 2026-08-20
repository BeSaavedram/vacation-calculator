package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// SaldoDeTipo es el saldo de un tipo tal como lo consume la interfaz.
type SaldoDeTipo struct {
	TipoID      uuid.UUID               `json:"tipo_id"`
	TipoCodigo  string                  `json:"tipo_codigo"`
	TipoNombre  string                  `json:"tipo_nombre"`
	Disponible  decimal.Decimal         `json:"disponible"`
	Pendiente   decimal.Decimal         `json:"pendiente"`
	Solicitable decimal.Decimal         `json:"solicitable"`
	PorVencer   []domain.BolsaPorVencer `json:"por_vencer"`
}

// SaldoCompleto es la respuesta del endpoint de saldo. Los campos reservados a
// RRHH van como punteros: van en nil para un colaborador y el JSON los omite.
type SaldoCompleto struct {
	ColaboradorID     uuid.UUID       `json:"colaborador_id"`
	ColaboradorNombre string          `json:"colaborador_nombre"`
	FechaIngreso      time.Time       `json:"fecha_ingreso"`
	AlDia             time.Time       `json:"al_dia"`
	Total             decimal.Decimal `json:"total_disponible"`
	PorTipo           []SaldoDeTipo   `json:"por_tipo"`

	// Solo RRHH. El colaborador ve únicamente su disponible: así lo define el
	// Requisito 6.
	DevengadoNoOtorgado *decimal.Decimal     `json:"devengado_no_otorgado,omitempty"`
	Proporcional        *domain.Proporcional `json:"proporcional,omitempty"`
}

// Saldo proyecta el saldo de un colaborador sumando su ledger.
//
// El saldo no se lee de ninguna tabla. Se calcula acá, en cada consulta, a
// partir de los movimientos. Esa es la decisión central del sistema: un número
// que no existe no puede quedar desactualizado.
func (s *Servicio) Saldo(ctx context.Context, colaboradorID uuid.UUID, verComoRRHH bool) (SaldoCompleto, error) {
	hoy := s.Hoy()

	colaborador, err := store.ColaboradorPorID(ctx, s.Pool, s.EmpresaID, colaboradorID)
	if err != nil {
		return SaldoCompleto{}, err
	}
	bolsas, err := store.BolsasDeColaborador(ctx, s.Pool, s.EmpresaID, colaboradorID, nil)
	if err != nil {
		return SaldoCompleto{}, err
	}
	tipos, err := store.TiposPorID(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return SaldoCompleto{}, err
	}
	pendientes, err := store.DiasPendientesPorTipo(ctx, s.Pool, s.EmpresaID, colaboradorID)
	if err != nil {
		return SaldoCompleto{}, err
	}

	proyeccion := domain.ProyectarSaldo(colaboradorID, bolsas, tipos, hoy)

	out := SaldoCompleto{
		ColaboradorID:     colaborador.ID,
		ColaboradorNombre: colaborador.Nombre,
		FechaIngreso:      colaborador.FechaIngreso,
		AlDia:             hoy,
		Total:             proyeccion.Total(),
	}

	for _, t := range proyeccion.PorTipo {
		pendiente, existe := pendientes[t.TipoID]
		if !existe {
			pendiente = decimal.Zero
		}
		out.PorTipo = append(out.PorTipo, SaldoDeTipo{
			TipoID:      t.TipoID,
			TipoCodigo:  t.TipoCodigo,
			TipoNombre:  t.TipoNombre,
			Disponible:  t.Disponible,
			Pendiente:   pendiente,
			Solicitable: t.Disponible.Sub(pendiente),
			PorVencer:   t.PorVencer,
		})
	}

	if verComoRRHH {
		proporcional := domain.CalcularProporcional(colaborador, hoy)
		out.DevengadoNoOtorgado = &proporcional.Dias
		out.Proporcional = &proporcional
	}

	return out, nil
}

// Finiquito responde cuánto se le debe a un colaborador si se desvincula en la
// fecha dada. Es solo lectura: no escribe ningún movimiento.
func (s *Servicio) Finiquito(ctx context.Context, colaboradorID uuid.UUID, fecha time.Time) (domain.Finiquito, error) {
	colaborador, err := store.ColaboradorPorID(ctx, s.Pool, s.EmpresaID, colaboradorID)
	if err != nil {
		return domain.Finiquito{}, err
	}
	bolsas, err := store.BolsasDeColaborador(ctx, s.Pool, s.EmpresaID, colaboradorID, nil)
	if err != nil {
		return domain.Finiquito{}, err
	}
	tipos, err := store.TiposPorID(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return domain.Finiquito{}, err
	}

	pagables := make(map[uuid.UUID]bool, len(tipos))
	for id, t := range tipos {
		pagables[id] = t.PagableEnFiniquito
	}

	return domain.CalcularFiniquito(colaborador, bolsas, pagables, fecha), nil
}

// Historial devuelve el ledger de un colaborador, opcionalmente cortado a una
// fecha pasada.
func (s *Servicio) Historial(
	ctx context.Context, colaboradorID uuid.UUID, hasta *time.Time,
) ([]store.MovimientoConContexto, error) {
	return store.HistorialDeColaborador(ctx, s.Pool, s.EmpresaID, colaboradorID, hasta)
}
