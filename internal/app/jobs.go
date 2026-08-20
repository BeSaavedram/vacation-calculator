package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// ResultadoJob resume lo que hizo una corrida.
type ResultadoJob struct {
	Fecha      time.Time `json:"fecha"`
	Evaluados  int       `json:"evaluados"`
	Creados    int       `json:"creados"`
	YaExistian int       `json:"ya_existian"`
	Detalle    []string  `json:"detalle"`
}

// CorrerDevengo evalúa a todos los colaboradores contra todos los tipos con
// devengo automático y crea los otorgamientos que correspondan a esa fecha.
//
// Es idempotente y acepta fecha objetivo. Reejecutarlo para una fecha ya
// procesada no duplica nada: la clave de idempotencia colisiona. Correrlo con
// una fecha pasada recupera un día que no se procesó. Esas dos propiedades son
// las que convierten una falla silenciosa en algo recuperable.
func (s *Servicio) CorrerDevengo(ctx context.Context, fecha time.Time, actorID uuid.UUID) (ResultadoJob, error) {
	fecha = domain.SoloFecha(fecha)
	res := ResultadoJob{Fecha: fecha}

	colaboradores, err := store.ListarColaboradores(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return res, err
	}
	tipos, err := store.ListarTipos(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return res, err
	}

	for _, c := range colaboradores {
		if c.FechaTermino != nil && !fecha.Before(*c.FechaTermino) {
			continue
		}
		for _, tipo := range tipos {
			res.Evaluados++

			resultado, hubo := domain.Devengar(tipo, c, fecha)
			if !hubo {
				continue
			}

			creado, err := s.otorgar(ctx, c, tipo, resultado, domain.OrigenAutomatico, actorID, resultado.Detalle)
			if err != nil {
				return res, fmt.Errorf("devengando %s/%s: %w", c.Nombre, tipo.Codigo, err)
			}
			if creado {
				res.Creados++
				res.Detalle = append(res.Detalle,
					fmt.Sprintf("%s · %s · %s días", c.Nombre, tipo.Codigo, resultado.Dias))
			} else {
				res.YaExistian++
			}
		}
	}
	return res, nil
}

// otorgar crea la bolsa y su movimiento ACCRUAL en una transacción. Devuelve
// false si el movimiento ya existía por clave de idempotencia, en cuyo caso la
// transacción se revierte y no queda una bolsa huérfana.
func (s *Servicio) otorgar(
	ctx context.Context,
	c domain.Colaborador,
	tipo domain.TipoDeVacacion,
	resultado domain.ResultadoDevengo,
	origen domain.Origen,
	actorID uuid.UUID,
	motivo string,
) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	otorgamiento := domain.Otorgamiento{
		EmpresaID:     s.EmpresaID,
		ColaboradorID: c.ID,
		TipoID:        tipo.ID,
		PeriodoDesde:  resultado.PeriodoDesde,
		PeriodoHasta:  resultado.PeriodoHasta,
		DiasOtorgados: resultado.Dias,
		DevengadoEl:   resultado.DevengadoEl,
		VenceEl:       domain.CalcularVencimiento(tipo, resultado.DevengadoEl),
		Origen:        origen,
	}

	otorgamientoID, err := store.CrearOtorgamiento(ctx, tx, otorgamiento)
	if err != nil {
		return false, err
	}

	clave := domain.ClaveIdempotencia(domain.ClaseAccrual, c.ID, tipo.ID, resultado.PeriodoDesde)
	if origen == domain.OrigenManual {
		// Un otorgamiento manual puede repetirse legítimamente el mismo día,
		// así que su clave incluye el id de la bolsa recién creada.
		clave = domain.ClaveIdempotenciaBolsa(domain.ClaseAccrual, otorgamientoID, resultado.DevengadoEl)
	}

	insertado, err := store.InsertarMovimiento(ctx, tx, domain.Movimiento{
		EmpresaID:         s.EmpresaID,
		OtorgamientoID:    otorgamientoID,
		Cantidad:          resultado.Dias,
		Clase:             domain.ClaseAccrual,
		FechaEfectiva:     resultado.DevengadoEl,
		ActorID:           actorID,
		Motivo:            motivo,
		ClaveIdempotencia: clave,
	})
	if err != nil {
		return false, err
	}
	if !insertado {
		// El otorgamiento ya existía: revertimos para no dejar una bolsa vacía.
		return false, nil
	}

	return true, tx.Commit(ctx)
}

// CorrerVencimiento hace vencer las bolsas cuya fecha de vencimiento ya llegó,
// emitiendo un EXPIRATION por el remanente exacto de cada una.
//
// También es idempotente: la clave incluye la bolsa y su fecha de vencimiento.
func (s *Servicio) CorrerVencimiento(ctx context.Context, fecha time.Time, actorID uuid.UUID) (ResultadoJob, error) {
	fecha = domain.SoloFecha(fecha)
	res := ResultadoJob{Fecha: fecha}

	colaboradores, err := store.ListarColaboradores(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return res, err
	}

	for _, c := range colaboradores {
		bolsas, err := store.BolsasDeColaborador(ctx, s.Pool, s.EmpresaID, c.ID, nil)
		if err != nil {
			return res, err
		}

		for _, b := range bolsas {
			res.Evaluados++

			vence := b.Otorgamiento.VenceEl
			if vence == nil || fecha.Before(*vence) {
				continue
			}
			remanente := b.Remanente()
			if !remanente.IsPositive() {
				continue
			}

			insertado, err := store.InsertarMovimiento(ctx, s.Pool, domain.Movimiento{
				EmpresaID:      s.EmpresaID,
				OtorgamientoID: b.Otorgamiento.ID,
				Cantidad:       remanente.Neg(),
				Clase:          domain.ClaseExpiration,
				FechaEfectiva:  *vence,
				ActorID:        actorID,
				Motivo: fmt.Sprintf("vencimiento automático: %s días no utilizados del período %s",
					remanente, b.Otorgamiento.PeriodoDesde.Format("2006-01-02")),
				ClaveIdempotencia: domain.ClaveIdempotenciaBolsa(
					domain.ClaseExpiration, b.Otorgamiento.ID, *vence),
			})
			if err != nil {
				return res, err
			}
			if insertado {
				res.Creados++
				res.Detalle = append(res.Detalle,
					fmt.Sprintf("%s · %s días vencidos el %s", c.Nombre, remanente, vence.Format("2006-01-02")))
			} else {
				res.YaExistian++
			}
		}
	}
	return res, nil
}
