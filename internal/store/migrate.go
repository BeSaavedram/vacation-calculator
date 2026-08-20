package store

import (
	"context"
	"embed"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migraciones embed.FS

// Migrar aplica los archivos SQL de migrations/ en orden alfabético, saltando
// los que ya se aplicaron. Correrlo dos veces es seguro.
//
// Debe correr con el rol dueño de la base, no con el rol de aplicación: el rol
// de aplicación deliberadamente no tiene permisos de DDL.
func Migrar(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS migracion_aplicada (
			nombre     text PRIMARY KEY,
			aplicada_el timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("creando el registro de migraciones: %w", err)
	}

	entradas, err := migraciones.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("leyendo migraciones: %w", err)
	}

	nombres := make([]string, 0, len(entradas))
	for _, e := range entradas {
		nombres = append(nombres, e.Name())
	}
	sort.Strings(nombres)

	for _, nombre := range nombres {
		var yaEsta bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM migracion_aplicada WHERE nombre = $1)`,
			nombre).Scan(&yaEsta); err != nil {
			return err
		}
		if yaEsta {
			fmt.Printf("omitida  %s (ya aplicada)\n", nombre)
			continue
		}

		sql, err := migraciones.ReadFile("migrations/" + nombre)
		if err != nil {
			return fmt.Errorf("leyendo %s: %w", nombre, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("aplicando %s: %w", nombre, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO migracion_aplicada (nombre) VALUES ($1)`, nombre); err != nil {
			return err
		}
		fmt.Printf("aplicada %s\n", nombre)
	}
	return nil
}
