package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// DiasPorMes es la tasa de devengo continuo: 15 días hábiles al año divididos
// en 12 meses.
var DiasPorMes = decimal.RequireFromString("1.25")

var treinta = decimal.NewFromInt(30)

// Proporcional es el derecho devengado del período en curso que todavía no se
// otorgó. Responde la pregunta de RRHH: "¿cuánto le debo si lo desvinculo hoy?".
//
// Es el mismo cálculo que alimenta el campo devengado_no_otorgado del saldo.
type Proporcional struct {
	PeriodoDesde   time.Time
	Hasta          time.Time
	MesesCompletos int
	DiasRestantes  int
	Dias           decimal.Decimal
}

// CalcularProporcional aplica meses × 1,25 + (días / 30) × 1,25 sobre el período
// en curso, prorrateando la fracción sobre base 30.
func CalcularProporcional(c Colaborador, fecha time.Time) Proporcional {
	fecha = SoloFecha(fecha)
	inicio := UltimoAniversario(c.FechaIngreso, fecha)

	meses := MesesEntre(inicio, fecha)
	baseDelMes := inicio.AddDate(0, meses, 0)
	dias := DiasEntre(baseDelMes, fecha)

	porMeses := DiasPorMes.Mul(decimal.NewFromInt(int64(meses)))
	porDias := decimal.NewFromInt(int64(dias)).Div(treinta).Mul(DiasPorMes)

	return Proporcional{
		PeriodoDesde:   inicio,
		Hasta:          fecha,
		MesesCompletos: meses,
		DiasRestantes:  dias,
		Dias:           porMeses.Add(porDias).Round(2),
	}
}

// Finiquito es el desglose completo de lo que se le debe a un colaborador si se
// desvincula en la fecha dada. Es una consulta de solo lectura: no escribe
// movimientos. El SETTLEMENT_PAYOUT se escribe recién al confirmarse la
// desvinculación, que está fuera del alcance de este MVP.
type Finiquito struct {
	Proporcional      Proporcional
	DisponiblePagable decimal.Decimal
	Total             decimal.Decimal
}

// CalcularFiniquito suma el proporcional del período en curso y el disponible de
// los tipos marcados como pagables.
func CalcularFiniquito(
	c Colaborador,
	bolsas []Bolsa,
	pagablePorTipo map[uuid.UUID]bool,
	fecha time.Time,
) Finiquito {
	proporcional := CalcularProporcional(c, fecha)

	disponible := decimal.Zero
	for _, b := range bolsas {
		if !pagablePorTipo[b.Otorgamiento.TipoID] {
			continue
		}
		if !b.VigenteAl(fecha) {
			continue
		}
		disponible = disponible.Add(b.Remanente())
	}

	return Finiquito{
		Proporcional:      proporcional,
		DisponiblePagable: disponible.Round(2),
		Total:             proporcional.Dias.Add(disponible).Round(2),
	}
}
