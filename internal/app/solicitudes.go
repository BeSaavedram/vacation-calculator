package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

var (
	// ErrRangoInvalido indica que el rango pedido no contiene días hábiles.
	ErrRangoInvalido = errors.New("el rango no contiene días hábiles")
	// ErrYaDecidida indica que la solicitud ya no está pendiente.
	ErrYaDecidida = errors.New("la solicitud ya fue decidida")
)

// PreviewSolicitud muestra cuántos días hábiles descuenta un rango antes de que
// el colaborador confirme.
type PreviewSolicitud struct {
	Desde        time.Time       `json:"desde"`
	Hasta        time.Time       `json:"hasta"`
	DiasHabiles  decimal.Decimal `json:"dias_habiles"`
	DiasCorridos int             `json:"dias_corridos"`
}

// PreviewDiasHabiles cuenta los días hábiles de un rango sin crear nada.
func (s *Servicio) PreviewDiasHabiles(ctx context.Context, desde, hasta time.Time) (PreviewSolicitud, error) {
	cal, err := s.Calendario(ctx)
	if err != nil {
		return PreviewSolicitud{}, err
	}
	habiles := cal.ContarHabiles(desde, hasta)

	return PreviewSolicitud{
		Desde:        domain.SoloFecha(desde),
		Hasta:        domain.SoloFecha(hasta),
		DiasHabiles:  decimal.NewFromInt(int64(habiles)),
		DiasCorridos: domain.DiasEntre(desde, hasta) + 1,
	}, nil
}

// CrearSolicitud registra la intención del colaborador. NO escribe movimientos:
// el ledger solo registra hechos consumados, y una solicitud pendiente todavía
// no lo es. Sí valida que el saldo alcance descontando lo ya comprometido en
// otras solicitudes pendientes.
func (s *Servicio) CrearSolicitud(
	ctx context.Context, colaboradorID, tipoID uuid.UUID, desde, hasta time.Time,
) (store.Solicitud, error) {
	preview, err := s.PreviewDiasHabiles(ctx, desde, hasta)
	if err != nil {
		return store.Solicitud{}, err
	}
	if !preview.DiasHabiles.IsPositive() {
		return store.Solicitud{}, ErrRangoInvalido
	}

	saldo, err := s.Saldo(ctx, colaboradorID, false)
	if err != nil {
		return store.Solicitud{}, err
	}

	solicitable := decimal.Zero
	for _, t := range saldo.PorTipo {
		if t.TipoID == tipoID {
			solicitable = t.Solicitable
		}
	}
	if preview.DiasHabiles.GreaterThan(solicitable) {
		return store.Solicitud{}, fmt.Errorf(
			"%w: pide %s días hábiles y tiene %s solicitables",
			domain.ErrSaldoInsuficiente, preview.DiasHabiles, solicitable)
	}

	id, err := store.CrearSolicitud(ctx, s.Pool, store.Solicitud{
		EmpresaID:     s.EmpresaID,
		ColaboradorID: colaboradorID,
		TipoID:        tipoID,
		Desde:         preview.Desde,
		Hasta:         preview.Hasta,
		DiasHabiles:   preview.DiasHabiles,
	})
	if err != nil {
		return store.Solicitud{}, err
	}
	return store.SolicitudPorID(ctx, s.Pool, s.EmpresaID, id)
}

// AprobarSolicitud es el único punto del sistema que escribe consumos.
//
// Todo ocurre en una transacción con bloqueo de fila sobre el colaborador: el
// bloqueo impide que dos aprobaciones concurrentes lean el mismo saldo y gasten
// los mismos días. Dentro, el asignador reparte FIFO entre las bolsas del tipo
// pedido y cada tramo genera su propio movimiento, de modo que el historial
// muestra de qué bolsa salió cada día.
func (s *Servicio) AprobarSolicitud(ctx context.Context, solicitudID, aprobadorID uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	solicitud, err := store.SolicitudPorID(ctx, tx, s.EmpresaID, solicitudID)
	if err != nil {
		return err
	}
	if solicitud.Estado != domain.EstadoPendiente {
		return ErrYaDecidida
	}

	if err := store.BloquearColaborador(ctx, tx, solicitud.ColaboradorID); err != nil {
		return err
	}

	bolsas, err := store.BolsasDeColaborador(ctx, tx, s.EmpresaID, solicitud.ColaboradorID, &solicitud.TipoID)
	if err != nil {
		return err
	}

	asignaciones, err := domain.AsignarConsumo(bolsas, solicitud.DiasHabiles, s.Hoy())
	if err != nil {
		return err
	}

	for i, a := range asignaciones {
		_, err := store.InsertarMovimiento(ctx, tx, domain.Movimiento{
			EmpresaID:      s.EmpresaID,
			OtorgamientoID: a.OtorgamientoID,
			SolicitudID:    &solicitud.ID,
			Cantidad:       a.Dias.Neg(),
			Clase:          domain.ClaseConsumption,
			FechaEfectiva:  solicitud.Desde,
			ActorID:        aprobadorID,
			Motivo: fmt.Sprintf("vacaciones %s al %s",
				solicitud.Desde.Format("2006-01-02"), solicitud.Hasta.Format("2006-01-02")),
			// El índice del tramo hace única la clave cuando una misma
			// solicitud se reparte entre varias bolsas.
			ClaveIdempotencia: fmt.Sprintf("CONSUMPTION:%s:%d", solicitud.ID, i),
		})
		if err != nil {
			return err
		}
	}

	decidida, err := store.DecidirSolicitud(ctx, tx, solicitud.ID,
		domain.EstadoAprobada, aprobadorID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !decidida {
		return ErrYaDecidida
	}

	return tx.Commit(ctx)
}

// RechazarSolicitud cierra la solicitud sin tocar el ledger. Los días
// comprometidos vuelven a estar solicitables automáticamente, porque el
// disponible solicitable se calcula descontando solo las pendientes.
func (s *Servicio) RechazarSolicitud(ctx context.Context, solicitudID, aprobadorID uuid.UUID) error {
	decidida, err := store.DecidirSolicitud(ctx, s.Pool, solicitudID,
		domain.EstadoRechazada, aprobadorID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !decidida {
		return ErrYaDecidida
	}
	return nil
}

// ListarSolicitudes devuelve las solicitudes visibles para quien consulta.
func (s *Servicio) ListarSolicitudes(ctx context.Context, actor domain.Colaborador) ([]store.Solicitud, error) {
	if actor.EsRRHH() {
		return store.ListarSolicitudes(ctx, s.Pool, s.EmpresaID, nil)
	}
	return store.ListarSolicitudes(ctx, s.Pool, s.EmpresaID, &actor.ID)
}
