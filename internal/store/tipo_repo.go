package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

const camposTipo = `
	id, empresa_id, codigo, nombre, politica_devengo, politica_vencimiento,
	parametros, prioridad_consumo, unidad_habil, pagable_en_finiquito, vigente_desde`

func escanearTipo(row pgx.Row) (domain.TipoDeVacacion, error) {
	var t domain.TipoDeVacacion
	var params []byte

	err := row.Scan(&t.ID, &t.EmpresaID, &t.Codigo, &t.Nombre,
		&t.PoliticaDevengo, &t.PoliticaVencimiento, &params,
		&t.PrioridadConsumo, &t.UnidadHabil, &t.PagableEnFiniquito, &t.VigenteDesde)
	if err != nil {
		return domain.TipoDeVacacion{}, err
	}
	if err := json.Unmarshal(params, &t.Parametros); err != nil {
		return domain.TipoDeVacacion{}, err
	}
	return t, nil
}

// ListarTipos devuelve los tipos de vacación de la empresa, ordenados por la
// prioridad con que se consumen.
func ListarTipos(ctx context.Context, q Querier, empresaID uuid.UUID) ([]domain.TipoDeVacacion, error) {
	rows, err := q.Query(ctx,
		`SELECT `+camposTipo+` FROM tipo_de_vacacion WHERE empresa_id = $1
		 ORDER BY prioridad_consumo, codigo`, empresaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.TipoDeVacacion
	for rows.Next() {
		t, err := escanearTipo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TiposPorID devuelve los tipos indexados por su id, que es la forma en que los
// consume el dominio.
func TiposPorID(ctx context.Context, q Querier, empresaID uuid.UUID) (map[uuid.UUID]domain.TipoDeVacacion, error) {
	tipos, err := ListarTipos(ctx, q, empresaID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]domain.TipoDeVacacion, len(tipos))
	for _, t := range tipos {
		out[t.ID] = t
	}
	return out, nil
}

// TipoPorCodigo busca un tipo por su código de negocio.
func TipoPorCodigo(ctx context.Context, q Querier, empresaID uuid.UUID, codigo string) (domain.TipoDeVacacion, error) {
	row := q.QueryRow(ctx,
		`SELECT `+camposTipo+` FROM tipo_de_vacacion WHERE empresa_id = $1 AND codigo = $2`,
		empresaID, codigo)
	return escanearTipo(row)
}

// CrearTipo inserta un tipo nuevo. Es lo que hace posible el Requisito 7:
// agregar "días por rendimiento" es esta llamada, no un despliegue.
func CrearTipo(ctx context.Context, q Querier, t domain.TipoDeVacacion) (uuid.UUID, error) {
	params, err := json.Marshal(t.Parametros)
	if err != nil {
		return uuid.Nil, err
	}

	var id uuid.UUID
	err = q.QueryRow(ctx, `
		INSERT INTO tipo_de_vacacion (empresa_id, codigo, nombre, politica_devengo,
			politica_vencimiento, parametros, prioridad_consumo, unidad_habil,
			pagable_en_finiquito, vigente_desde)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		t.EmpresaID, t.Codigo, t.Nombre, t.PoliticaDevengo, t.PoliticaVencimiento,
		params, t.PrioridadConsumo, t.UnidadHabil, t.PagableEnFiniquito, t.VigenteDesde,
	).Scan(&id)
	return id, err
}

// ActualizarTipo modifica la configuración de un tipo existente.
func ActualizarTipo(ctx context.Context, q Querier, t domain.TipoDeVacacion) error {
	params, err := json.Marshal(t.Parametros)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		UPDATE tipo_de_vacacion
		SET nombre = $3, politica_devengo = $4, politica_vencimiento = $5,
			parametros = $6, prioridad_consumo = $7, unidad_habil = $8,
			pagable_en_finiquito = $9
		WHERE empresa_id = $1 AND id = $2`,
		t.EmpresaID, t.ID, t.Nombre, t.PoliticaDevengo, t.PoliticaVencimiento,
		params, t.PrioridadConsumo, t.UnidadHabil, t.PagableEnFiniquito)
	return err
}
