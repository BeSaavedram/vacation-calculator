package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// Servicio agrupa los casos de uso. Recibe el pool y el id de la empresa, que
// en este MVP es única pero viaja explícito en cada consulta.
type Servicio struct {
	Pool      *pgxpool.Pool
	EmpresaID uuid.UUID
	// Hoy permite fijar la fecha actual desde afuera. En producción es
	// time.Now; en tests y en la semilla se sustituye para poder recorrer la
	// historia de un colaborador.
	Hoy func() time.Time
}

// NuevoServicio construye el servicio con el reloj real.
func NuevoServicio(pool *pgxpool.Pool, empresaID uuid.UUID) *Servicio {
	return &Servicio{
		Pool:      pool,
		EmpresaID: empresaID,
		Hoy:       func() time.Time { return domain.SoloFecha(time.Now().UTC()) },
	}
}

// Calendario carga el calendario laboral vigente.
func (s *Servicio) Calendario(ctx context.Context) (*domain.Calendario, error) {
	fechas, err := store.CargarCalendario(ctx, s.Pool, "CL")
	if err != nil {
		return nil, err
	}
	return domain.NuevoCalendario(fechas), nil
}
