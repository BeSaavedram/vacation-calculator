package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// BolsasDeColaborador arma las bolsas de un colaborador: cada otorgamiento con
// todos sus movimientos. El saldo NO se lee de ninguna columna; se calcula
// después sumando estos movimientos.
//
// Si tipoID no es nil, filtra a las bolsas de ese tipo.
func BolsasDeColaborador(
	ctx context.Context, q Querier, empresaID, colaboradorID uuid.UUID, tipoID *uuid.UUID,
) ([]domain.Bolsa, error) {
	rows, err := q.Query(ctx, `
		SELECT o.id, o.empresa_id, o.colaborador_id, o.tipo_id, o.periodo_desde,
		       o.periodo_hasta, o.dias_otorgados::text, o.devengado_el, o.vence_el,
		       o.origen, t.prioridad_consumo
		FROM otorgamiento o
		JOIN tipo_de_vacacion t ON t.id = o.tipo_id
		WHERE o.empresa_id = $1
		  AND o.colaborador_id = $2
		  AND ($3::uuid IS NULL OR o.tipo_id = $3)
		ORDER BY o.devengado_el`,
		empresaID, colaboradorID, tipoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bolsas := make([]domain.Bolsa, 0)
	indice := make(map[uuid.UUID]int)

	for rows.Next() {
		var o domain.Otorgamiento
		var dias string
		var prioridad int

		if err := rows.Scan(&o.ID, &o.EmpresaID, &o.ColaboradorID, &o.TipoID,
			&o.PeriodoDesde, &o.PeriodoHasta, &dias, &o.DevengadoEl, &o.VenceEl,
			&o.Origen, &prioridad); err != nil {
			return nil, err
		}
		o.DiasOtorgados = decimal.RequireFromString(dias)

		indice[o.ID] = len(bolsas)
		bolsas = append(bolsas, domain.Bolsa{Otorgamiento: o, Prioridad: prioridad})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(bolsas) == 0 {
		return bolsas, nil
	}

	ids := make([]uuid.UUID, 0, len(bolsas))
	for _, b := range bolsas {
		ids = append(ids, b.Otorgamiento.ID)
	}

	movs, err := q.Query(ctx, `
		SELECT id, empresa_id, otorgamiento_id, solicitud_id, cantidad::text, clase,
		       fecha_efectiva, fecha_registro, actor_id, motivo, clave_idempotencia, reversa_de
		FROM movimiento
		WHERE otorgamiento_id = ANY($1)
		ORDER BY fecha_efectiva, fecha_registro`, ids)
	if err != nil {
		return nil, err
	}
	defer movs.Close()

	for movs.Next() {
		var m domain.Movimiento
		var cantidad string

		if err := movs.Scan(&m.ID, &m.EmpresaID, &m.OtorgamientoID, &m.SolicitudID,
			&cantidad, &m.Clase, &m.FechaEfectiva, &m.FechaRegistro, &m.ActorID,
			&m.Motivo, &m.ClaveIdempotencia, &m.ReversaDe); err != nil {
			return nil, err
		}
		m.Cantidad = decimal.RequireFromString(cantidad)

		if i, ok := indice[m.OtorgamientoID]; ok {
			bolsas[i].Movimientos = append(bolsas[i].Movimientos, m)
		}
	}
	return bolsas, movs.Err()
}

// MovimientoConContexto es una fila del historial tal como se muestra en la
// interfaz: el movimiento más el tipo y el actor, resueltos.
type MovimientoConContexto struct {
	domain.Movimiento
	TipoCodigo  string
	TipoNombre  string
	ActorNombre string
	VenceEl     *time.Time
}

// HistorialDeColaborador devuelve el ledger completo del colaborador, en orden
// cronológico. Con `hasta` distinto de nil, corta por fecha efectiva: eso es lo
// que permite reconstruir y explicar el saldo a cualquier fecha pasada.
func HistorialDeColaborador(
	ctx context.Context, q Querier, empresaID, colaboradorID uuid.UUID, hasta *time.Time,
) ([]MovimientoConContexto, error) {
	rows, err := q.Query(ctx, `
		SELECT m.id, m.empresa_id, m.otorgamiento_id, m.solicitud_id, m.cantidad::text,
		       m.clase, m.fecha_efectiva, m.fecha_registro, m.actor_id, m.motivo,
		       m.clave_idempotencia, m.reversa_de,
		       t.codigo, t.nombre, a.nombre, o.vence_el
		FROM movimiento m
		JOIN otorgamiento o ON o.id = m.otorgamiento_id
		JOIN tipo_de_vacacion t ON t.id = o.tipo_id
		JOIN colaborador a ON a.id = m.actor_id
		WHERE m.empresa_id = $1
		  AND o.colaborador_id = $2
		  AND ($3::date IS NULL OR m.fecha_efectiva <= $3)
		ORDER BY m.fecha_efectiva, m.fecha_registro`,
		empresaID, colaboradorID, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MovimientoConContexto
	for rows.Next() {
		var m MovimientoConContexto
		var cantidad string

		if err := rows.Scan(&m.ID, &m.EmpresaID, &m.OtorgamientoID, &m.SolicitudID,
			&cantidad, &m.Clase, &m.FechaEfectiva, &m.FechaRegistro, &m.ActorID,
			&m.Motivo, &m.ClaveIdempotencia, &m.ReversaDe,
			&m.TipoCodigo, &m.TipoNombre, &m.ActorNombre, &m.VenceEl); err != nil {
			return nil, err
		}
		m.Cantidad = decimal.RequireFromString(cantidad)
		out = append(out, m)
	}
	return out, rows.Err()
}

// CrearOtorgamiento inserta una bolsa nueva.
func CrearOtorgamiento(ctx context.Context, q Querier, o domain.Otorgamiento) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO otorgamiento (empresa_id, colaborador_id, tipo_id, periodo_desde,
			periodo_hasta, dias_otorgados, devengado_el, vence_el, origen)
		VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $8, $9)
		RETURNING id`,
		o.EmpresaID, o.ColaboradorID, o.TipoID, o.PeriodoDesde, o.PeriodoHasta,
		o.DiasOtorgados.String(), o.DevengadoEl, o.VenceEl, o.Origen,
	).Scan(&id)
	return id, err
}

// InsertarMovimiento escribe una fila en el ledger. Devuelve false si la clave
// de idempotencia ya existía, en cuyo caso NO se insertó nada.
//
// Esto es lo que hace que reejecutar un job sea seguro: la segunda corrida
// colisiona contra el índice único y no duplica movimientos.
func InsertarMovimiento(ctx context.Context, q Querier, m domain.Movimiento) (bool, error) {
	// InsertarMovimiento es el único punto por el que un movimiento entra al
	// ledger, y el ledger nunca se actualiza ni se borra (la base solo otorga
	// SELECT e INSERT sobre esta tabla). Un signo equivocado no se puede
	// corregir después, así que se valida acá, una sola vez, para todo llamador
	// presente y futuro.
	if err := m.Validar(); err != nil {
		return false, err
	}

	tag, err := q.Exec(ctx, `
		INSERT INTO movimiento (empresa_id, otorgamiento_id, solicitud_id, cantidad,
			clase, fecha_efectiva, actor_id, motivo, clave_idempotencia, reversa_de)
		VALUES ($1, $2, $3, $4::numeric, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (clave_idempotencia) DO NOTHING`,
		m.EmpresaID, m.OtorgamientoID, m.SolicitudID, m.Cantidad.String(),
		m.Clase, m.FechaEfectiva, m.ActorID, m.Motivo, m.ClaveIdempotencia, m.ReversaDe)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
