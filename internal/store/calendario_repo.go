package store

import (
	"context"
	"time"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// CargarCalendario trae las fechas inhábiles de un ámbito. Se carga completo:
// son unos cientos de filas y se consulta en cada cálculo de días hábiles.
func CargarCalendario(ctx context.Context, q Querier, ambito string) ([]time.Time, error) {
	rows, err := q.Query(ctx,
		`SELECT fecha FROM calendario_laboral WHERE ambito = $1 ORDER BY fecha`, ambito)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var f time.Time
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append(out, domain.SoloFecha(f))
	}
	return out, rows.Err()
}

// Feriado es una fila del calendario laboral.
type Feriado struct {
	Fecha  time.Time
	Ambito string
	Tipo   string
	Nombre string
}

// InsertarFeriados carga el calendario. Solo lo usa la semilla. Es idempotente:
// reejecutarla no duplica fechas.
func InsertarFeriados(ctx context.Context, q Querier, feriados []Feriado) error {
	for _, f := range feriados {
		_, err := q.Exec(ctx, `
			INSERT INTO calendario_laboral (fecha, ambito, tipo, nombre)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (fecha, ambito) DO NOTHING`,
			f.Fecha, f.Ambito, f.Tipo, f.Nombre)
		if err != nil {
			return err
		}
	}
	return nil
}
