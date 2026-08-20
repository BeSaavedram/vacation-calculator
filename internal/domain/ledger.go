package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Otorgamiento es un lote concreto de días: la unidad sobre la que operan el
// vencimiento y el consumo FIFO.
type Otorgamiento struct {
	ID            uuid.UUID
	EmpresaID     uuid.UUID
	ColaboradorID uuid.UUID
	TipoID        uuid.UUID
	PeriodoDesde  time.Time
	PeriodoHasta  time.Time
	DiasOtorgados decimal.Decimal
	DevengadoEl   time.Time
	VenceEl       *time.Time
	Origen        Origen
}

// Movimiento es el registro inmutable de todo cambio de saldo. Nunca se
// actualiza ni se borra: una corrección es un REVERSAL más un movimiento nuevo.
//
// FechaEfectiva es cuándo ocurrió el hecho; FechaRegistro es cuándo lo supimos.
// Guardar ambas permite responder "¿qué sabíamos el 3 de marzo?" y no solo
// "¿qué es cierto hoy?".
type Movimiento struct {
	ID                uuid.UUID
	EmpresaID         uuid.UUID
	OtorgamientoID    uuid.UUID
	SolicitudID       *uuid.UUID
	Cantidad          decimal.Decimal
	Clase             ClaseMovimiento
	FechaEfectiva     time.Time
	FechaRegistro     time.Time
	ActorID           uuid.UUID
	Motivo            string
	ClaveIdempotencia string
	ReversaDe         *uuid.UUID
}

// Bolsa es un otorgamiento junto a los movimientos que lo afectaron. El saldo
// de la bolsa no se guarda: es la suma de sus movimientos.
type Bolsa struct {
	Otorgamiento Otorgamiento
	Movimientos  []Movimiento
	// Prioridad se copia desde el TipoDeVacacion al armar la bolsa. Vive aquí
	// para que el ordenamiento FIFO no necesite cargar el tipo completo.
	Prioridad int
}

// Remanente es lo que queda en la bolsa: la suma con signo de sus movimientos.
func (b Bolsa) Remanente() decimal.Decimal {
	total := decimal.Zero
	for _, m := range b.Movimientos {
		total = total.Add(m.Cantidad)
	}
	return total
}

// VigenteAl indica si la bolsa todavía puede consumirse en la fecha dada.
// vence_el es exclusivo: el día del vencimiento la bolsa ya no sirve.
func (b Bolsa) VigenteAl(fecha time.Time) bool {
	if b.Otorgamiento.VenceEl == nil {
		return true
	}
	// Ambos lados normalizados: vence_el viene de la base y puede traer zona.
	// Comparar contra el instante crudo hacía que una bolsa ya vencida se
	// reportara como usable.
	return SoloFecha(fecha).Before(SoloFecha(*b.Otorgamiento.VenceEl))
}

// ClaveIdempotencia construye la clave única que impide duplicar un movimiento
// automático. Es UNIQUE en base de datos: reejecutar un job no duplica nada
// porque el INSERT colisiona.
func ClaveIdempotencia(clase ClaseMovimiento, colaboradorID, tipoID uuid.UUID, periodo time.Time) string {
	return fmt.Sprintf("%s:%s:%s:%s", clase, colaboradorID, tipoID,
		SoloFecha(periodo).Format("2006-01-02"))
}

// ClaveIdempotenciaBolsa construye la clave para un movimiento que se refiere a
// una bolsa concreta, como el vencimiento.
func ClaveIdempotenciaBolsa(clase ClaseMovimiento, otorgamientoID uuid.UUID, fecha time.Time) string {
	return fmt.Sprintf("%s:%s:%s", clase, otorgamientoID, SoloFecha(fecha).Format("2006-01-02"))
}
