package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// DiasParaAvisoDeVencimiento define la ventana en la que una bolsa se considera
// "próxima a vencer" y se destaca en la interfaz.
const DiasParaAvisoDeVencimiento = 90

// BolsaPorVencer es un lote de días que está por perderse.
type BolsaPorVencer struct {
	OtorgamientoID uuid.UUID
	Dias           decimal.Decimal
	VenceEl        time.Time
}

// SaldoPorTipo es el disponible de un tipo de vacación concreto.
type SaldoPorTipo struct {
	TipoID     uuid.UUID
	TipoCodigo string
	TipoNombre string
	Disponible decimal.Decimal
	PorVencer  []BolsaPorVencer
}

// Saldo es una PROYECCIÓN, no un dato almacenado. Se recalcula sumando el
// ledger en cada consulta. No existe en ninguna tabla como número editable:
// esa es la decisión central de este sistema.
type Saldo struct {
	ColaboradorID uuid.UUID
	AlDia         time.Time
	PorTipo       []SaldoPorTipo
}

// Total suma el disponible de todos los tipos.
func (s Saldo) Total() decimal.Decimal {
	total := decimal.Zero
	for _, t := range s.PorTipo {
		total = total.Add(t.Disponible)
	}
	return total
}

// ProyectarSaldo reduce las bolsas de un colaborador a su saldo disponible por
// tipo, descartando lo vencido y destacando lo que está por vencer.
//
// colaboradorID se recibe y NO se infiere de las bolsas: cero bolsas es el
// estado normal de todo recién contratado, y ahí no hay nada de dónde inferirlo.
// El llamador siempre tiene este dato.
//
// alDia decide ÚNICAMENTE el vencimiento y la ventana de aviso. El remanente de
// cada bolsa sigue siendo la suma completa de su ledger, incluidos los consumos
// con fecha efectiva futura: unas vacaciones aprobadas para el mes que viene ya
// están comprometidas y no deben poder gastarse dos veces.
func ProyectarSaldo(
	colaboradorID uuid.UUID,
	bolsas []Bolsa,
	tipos map[uuid.UUID]TipoDeVacacion,
	alDia time.Time,
) Saldo {
	alDia = SoloFecha(alDia)
	limiteAviso := alDia.AddDate(0, 0, DiasParaAvisoDeVencimiento)

	porTipo := make(map[uuid.UUID]*SaldoPorTipo)

	for _, b := range bolsas {
		remanente, aporta := disponibleDeBolsa(b, alDia)
		if !aporta {
			continue
		}

		tipoID := b.Otorgamiento.TipoID
		entrada, existe := porTipo[tipoID]
		if !existe {
			tipo := tipos[tipoID]
			entrada = &SaldoPorTipo{
				TipoID:     tipoID,
				TipoCodigo: tipo.Codigo,
				TipoNombre: tipo.Nombre,
				Disponible: decimal.Zero,
			}
			porTipo[tipoID] = entrada
		}

		entrada.Disponible = entrada.Disponible.Add(remanente)

		if v := b.Otorgamiento.VenceEl; v != nil && v.Before(limiteAviso) {
			entrada.PorVencer = append(entrada.PorVencer, BolsaPorVencer{
				OtorgamientoID: b.Otorgamiento.ID,
				Dias:           remanente,
				VenceEl:        *v,
			})
		}
	}

	saldo := Saldo{ColaboradorID: colaboradorID, AlDia: alDia}
	for _, entrada := range porTipo {
		sort.Slice(entrada.PorVencer, func(i, j int) bool {
			return entrada.PorVencer[i].VenceEl.Before(entrada.PorVencer[j].VenceEl)
		})
		saldo.PorTipo = append(saldo.PorTipo, *entrada)
	}

	// Orden estable por prioridad de consumo: primero lo que se gasta primero.
	sort.Slice(saldo.PorTipo, func(i, j int) bool {
		pi := tipos[saldo.PorTipo[i].TipoID].PrioridadConsumo
		pj := tipos[saldo.PorTipo[j].TipoID].PrioridadConsumo
		if pi != pj {
			return pi < pj
		}
		return saldo.PorTipo[i].TipoCodigo < saldo.PorTipo[j].TipoCodigo
	})

	return saldo
}
