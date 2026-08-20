package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

const camposColaborador = `
	id, empresa_id, nombre, email, rol, fecha_ingreso, fecha_termino,
	meses_experiencia_previa, jornada`

func escanearColaborador(row pgx.Row) (domain.Colaborador, error) {
	var c domain.Colaborador
	err := row.Scan(&c.ID, &c.EmpresaID, &c.Nombre, &c.Email, &c.Rol,
		&c.FechaIngreso, &c.FechaTermino, &c.MesesExperienciaPrevia, &c.Jornada)
	if err != nil {
		return domain.Colaborador{}, err
	}
	c.FechaIngreso = domain.SoloFecha(c.FechaIngreso)
	return c, nil
}

// ColaboradorPorID busca un colaborador dentro de su empresa. El filtro por
// empresa_id es obligatorio en todos los repositorios, sin excepción.
func ColaboradorPorID(ctx context.Context, q Querier, empresaID, id uuid.UUID) (domain.Colaborador, error) {
	row := q.QueryRow(ctx,
		`SELECT `+camposColaborador+` FROM colaborador WHERE empresa_id = $1 AND id = $2`,
		empresaID, id)

	c, err := escanearColaborador(row)
	if err != nil {
		return domain.Colaborador{}, fmt.Errorf("colaborador %s: %w", id, err)
	}
	return c, nil
}

// ColaboradorPorIDSinEmpresa se usa solo en el middleware de actor, donde
// todavía no se sabe a qué empresa pertenece quien llama.
func ColaboradorPorIDSinEmpresa(ctx context.Context, q Querier, id uuid.UUID) (domain.Colaborador, error) {
	row := q.QueryRow(ctx, `SELECT `+camposColaborador+` FROM colaborador WHERE id = $1`, id)
	return escanearColaborador(row)
}

// ListarColaboradores devuelve todos los colaboradores de la empresa.
func ListarColaboradores(ctx context.Context, q Querier, empresaID uuid.UUID) ([]domain.Colaborador, error) {
	rows, err := q.Query(ctx,
		`SELECT `+camposColaborador+` FROM colaborador WHERE empresa_id = $1 ORDER BY nombre`,
		empresaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Colaborador
	for rows.Next() {
		c, err := escanearColaborador(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CrearColaborador inserta un colaborador. Solo lo usa la semilla.
func CrearColaborador(ctx context.Context, q Querier, c domain.Colaborador) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO colaborador (empresa_id, nombre, email, rol, fecha_ingreso,
			fecha_termino, meses_experiencia_previa, jornada)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		c.EmpresaID, c.Nombre, c.Email, c.Rol, c.FechaIngreso,
		c.FechaTermino, c.MesesExperienciaPrevia, c.Jornada,
	).Scan(&id)
	return id, err
}

// BloquearColaborador toma un lock de fila sobre el colaborador. Se llama al
// inicio de la transacción de aprobación para impedir que dos solicitudes
// concurrentes gasten los mismos días.
func BloquearColaborador(ctx context.Context, q Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `SELECT id FROM colaborador WHERE id = $1 FOR UPDATE`, id)
	return err
}
