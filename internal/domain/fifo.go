package domain

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ErrSaldoInsuficiente indica que las bolsas vigentes no alcanzan a cubrir los
// días pedidos.
var ErrSaldoInsuficiente = errors.New("saldo insuficiente")

// Asignacion es un tramo de consumo contra una bolsa concreta. Cada asignación
// se convertirá en un movimiento CONSUMPTION propio, para que el historial
// muestre de qué bolsa salió cada día.
type Asignacion struct {
	OtorgamientoID uuid.UUID
	Dias           decimal.Decimal
}

// AsignarConsumo reparte los días pedidos entre las bolsas, consumiendo primero
// las que vencen antes y desempatando por prioridad del tipo.
//
// Recibe las bolsas ya filtradas por tipo de vacación: la solicitud registra su
// tipo y el reparto ocurre dentro de ese tipo.
func AsignarConsumo(bolsas []Bolsa, dias decimal.Decimal, alDia time.Time) ([]Asignacion, error) {
	if dias.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("los días a consumir deben ser positivos")
	}

	candidatas := bolsasConsumibles(bolsas, alDia)
	ordenarFIFO(candidatas)

	restante := dias
	asignaciones := make([]Asignacion, 0, len(candidatas))

	for _, b := range candidatas {
		if restante.LessThanOrEqual(decimal.Zero) {
			break
		}
		tramo := decimal.Min(b.Remanente(), restante)
		asignaciones = append(asignaciones, Asignacion{
			OtorgamientoID: b.Otorgamiento.ID,
			Dias:           tramo,
		})
		restante = restante.Sub(tramo)
	}

	if restante.GreaterThan(decimal.Zero) {
		return nil, ErrSaldoInsuficiente
	}
	return asignaciones, nil
}

// bolsasConsumibles descarta las vencidas y las que ya no tienen remanente.
func bolsasConsumibles(bolsas []Bolsa, alDia time.Time) []Bolsa {
	out := make([]Bolsa, 0, len(bolsas))
	for _, b := range bolsas {
		if b.VigenteAl(alDia) && b.Remanente().GreaterThan(decimal.Zero) {
			out = append(out, b)
		}
	}
	return out
}

// ordenarFIFO ordena por vence_el ascendente y, a igual fecha, por prioridad.
// Las bolsas sin vencimiento van al final: se consumen cuando ya no queda nada
// que esté por perderse.
func ordenarFIFO(bolsas []Bolsa) {
	sort.SliceStable(bolsas, func(i, j int) bool {
		vi, vj := bolsas[i].Otorgamiento.VenceEl, bolsas[j].Otorgamiento.VenceEl

		switch {
		case vi == nil && vj == nil:
			return bolsas[i].Prioridad < bolsas[j].Prioridad
		case vi == nil:
			return false
		case vj == nil:
			return true
		case !vi.Equal(*vj):
			return vi.Before(*vj)
		default:
			return bolsas[i].Prioridad < bolsas[j].Prioridad
		}
	})
}
