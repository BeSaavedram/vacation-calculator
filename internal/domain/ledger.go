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
//
// CONVENCIÓN DE SIGNOS — es lo que hace funcionar todo el ledger, porque
// Remanente() suma con signo y no interpreta la clase.
//
// Clases de signo FIJO, porque la clase ya dice en qué dirección mueven el
// saldo; el signo contrario solo puede ser un error de quien escribe:
//
//	ACCRUAL, OPENING_BALANCE ................... Cantidad POSITIVA (suman días)
//	CONSUMPTION, EXPIRATION, SETTLEMENT_PAYOUT . Cantidad NEGATIVA (restan días)
//
// Clases de signo LIBRE, porque la dirección no está determinada por la clase
// sino por el hecho que registran:
//
//	ADJUSTMENT ... una corrección de RRHH puede agregar días o quitarlos
//	REVERSAL ..... refleja el movimiento que revierte, con el signo opuesto
//
// Cero nunca es válido en ninguna clase: un movimiento sin efecto no se registra.
//
// Un signo equivocado no se puede corregir reejecutando nada, así que Validar()
// debe llamarse antes de persistir cualquier movimiento.
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

// Validar verifica que el signo de la cantidad corresponda a la clase del
// movimiento, según la convención documentada en Movimiento.
func (m Movimiento) Validar() error {
	if m.Cantidad.IsZero() {
		return fmt.Errorf("cantidad cero en un movimiento %s: un movimiento sin efecto no se registra", m.Clase)
	}

	switch m.Clase {
	case ClaseAccrual, ClaseOpening:
		if m.Cantidad.IsNegative() {
			return fmt.Errorf("%s debe tener cantidad positiva, se recibió %s", m.Clase, m.Cantidad)
		}
	case ClaseConsumption, ClaseExpiration, ClasePayout:
		if m.Cantidad.IsPositive() {
			return fmt.Errorf("%s debe tener cantidad negativa, se recibió %s", m.Clase, m.Cantidad)
		}
	case ClaseAdjustment, ClaseReversal:
		// Signo libre: un ADJUSTMENT puede agregar o quitar días, y un REVERSAL
		// refleja el signo opuesto del movimiento que revierte.
	default:
		return fmt.Errorf("clase de movimiento desconocida: %q", m.Clase)
	}
	return nil
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

// Remanente es lo que queda en la bolsa: la suma con signo de TODOS sus
// movimientos, sin filtrar por fecha.
//
// Incluir los consumos con fecha efectiva futura es deliberado: unas vacaciones
// ya aprobadas para el mes que viene están comprometidas y no pueden volver a
// gastarse. "Lo que todavía puedo tomarme" es justamente el ledger completo.
func (b Bolsa) Remanente() decimal.Decimal {
	total := decimal.Zero
	for _, m := range b.Movimientos {
		total = total.Add(m.Cantidad)
	}
	return total
}

// disponibleDeBolsa dice cuánto aporta una bolsa al disponible, y si aporta.
//
// Es la ÚNICA definición de "disponible" del dominio: la usan tanto la
// proyección de saldo como el finiquito, para que RRHH no pueda ver dos cifras
// distintas de la misma persona el mismo día.
//
// Una bolsa aporta solo si está vigente y su remanente es positivo. Un remanente
// negativo es una ANOMALÍA de datos (se consumió más de lo otorgado), no un
// descuento legítimo: netearlo contra las bolsas sanas haría que el total
// cuadrara por casualidad y taparía el problema justo donde hay que verlo.
func disponibleDeBolsa(b Bolsa, alDia time.Time) (decimal.Decimal, bool) {
	if !b.VigenteAl(alDia) {
		return decimal.Zero, false
	}
	remanente := b.Remanente()
	if remanente.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, false
	}
	return remanente, true
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
