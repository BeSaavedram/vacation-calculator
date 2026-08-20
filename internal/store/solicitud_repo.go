package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// Solicitud es una solicitud de vacaciones con los datos que necesita la
// interfaz ya resueltos.
type Solicitud struct {
	ID             uuid.UUID
	EmpresaID      uuid.UUID
	ColaboradorID  uuid.UUID
	ColaboradorNom string
	TipoID         uuid.UUID
	TipoCodigo     string
	Desde          time.Time
	Hasta          time.Time
	DiasHabiles    decimal.Decimal
	Estado         domain.EstadoSolicitud
	AprobadorID    *uuid.UUID
	DecididoEl     *time.Time
	CreadaEl       time.Time
}

const selectSolicitud = `
	SELECT s.id, s.empresa_id, s.colaborador_id, c.nombre, s.tipo_id, t.codigo,
	       s.desde, s.hasta, s.dias_habiles::text, s.estado, s.aprobador_id,
	       s.decidido_el, s.creada_el
	FROM solicitud_de_vacaciones s
	JOIN colaborador c ON c.id = s.colaborador_id
	JOIN tipo_de_vacacion t ON t.id = s.tipo_id`

func escanearSolicitud(row pgx.Row) (Solicitud, error) {
	var s Solicitud
	var dias string
	err := row.Scan(&s.ID, &s.EmpresaID, &s.ColaboradorID, &s.ColaboradorNom,
		&s.TipoID, &s.TipoCodigo, &s.Desde, &s.Hasta, &dias, &s.Estado,
		&s.AprobadorID, &s.DecididoEl, &s.CreadaEl)
	if err != nil {
		return Solicitud{}, err
	}
	s.DiasHabiles = decimal.RequireFromString(dias)
	return s, nil
}

// CrearSolicitud inserta una solicitud en estado PENDIENTE.
func CrearSolicitud(ctx context.Context, q Querier, s Solicitud) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO solicitud_de_vacaciones (empresa_id, colaborador_id, tipo_id,
			desde, hasta, dias_habiles, estado)
		VALUES ($1, $2, $3, $4, $5, $6::numeric, $7)
		RETURNING id`,
		s.EmpresaID, s.ColaboradorID, s.TipoID, s.Desde, s.Hasta,
		s.DiasHabiles.String(), domain.EstadoPendiente,
	).Scan(&id)
	return id, err
}

// SolicitudPorID busca una solicitud dentro de su empresa.
func SolicitudPorID(ctx context.Context, q Querier, empresaID, id uuid.UUID) (Solicitud, error) {
	row := q.QueryRow(ctx, selectSolicitud+` WHERE s.empresa_id = $1 AND s.id = $2`, empresaID, id)
	return escanearSolicitud(row)
}

// ListarSolicitudes devuelve las solicitudes de la empresa. Con colaboradorID
// distinto de nil, solo las de ese colaborador.
func ListarSolicitudes(
	ctx context.Context, q Querier, empresaID uuid.UUID, colaboradorID *uuid.UUID,
) ([]Solicitud, error) {
	rows, err := q.Query(ctx, selectSolicitud+`
		WHERE s.empresa_id = $1
		  AND ($2::uuid IS NULL OR s.colaborador_id = $2)
		ORDER BY s.creada_el DESC`, empresaID, colaboradorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Solicitud
	for rows.Next() {
		s, err := escanearSolicitud(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DecidirSolicitud cambia el estado de una solicitud pendiente. La condición
// sobre el estado actual es la que impide aprobar dos veces la misma solicitud.
func DecidirSolicitud(
	ctx context.Context, q Querier, id uuid.UUID,
	estado domain.EstadoSolicitud, aprobadorID uuid.UUID, cuando time.Time,
) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE solicitud_de_vacaciones
		SET estado = $2, aprobador_id = $3, decidido_el = $4
		WHERE id = $1 AND estado = 'PENDIENTE'`,
		id, estado, aprobadorID, cuando)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DiasPendientesPorTipo suma los días comprometidos en solicitudes que todavía
// no se aprueban ni se rechazan.
//
// Estos días NO están en el ledger: el ledger solo registra hechos consumados.
// Pero se descuentan del disponible que se ofrece para solicitar, de modo que
// un colaborador no pueda comprometer dos veces los mismos días.
func DiasPendientesPorTipo(
	ctx context.Context, q Querier, empresaID, colaboradorID uuid.UUID,
) (map[uuid.UUID]decimal.Decimal, error) {
	rows, err := q.Query(ctx, `
		SELECT tipo_id, SUM(dias_habiles)::text
		FROM solicitud_de_vacaciones
		WHERE empresa_id = $1 AND colaborador_id = $2 AND estado = 'PENDIENTE'
		GROUP BY tipo_id`, empresaID, colaboradorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]decimal.Decimal)
	for rows.Next() {
		var tipoID uuid.UUID
		var suma string
		if err := rows.Scan(&tipoID, &suma); err != nil {
			return nil, err
		}
		out[tipoID] = decimal.RequireFromString(suma)
	}
	return out, rows.Err()
}
