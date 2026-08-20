package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// diasPorMes es la tasa de devengo continuo: 15 días hábiles al año divididos en
// 12 meses.
//
// Es una constante de la especificación, NO se deriva de DiasBase. Un tipo
// configurado con un DiasBase distinto de 15 no haría variar esta tasa: el
// proporcional de finiquito seguiría prorrateando 1,25 al mes.
var diasPorMes = decimal.RequireFromString("1.25")

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
	ingreso := SoloFecha(c.FechaIngreso)

	// Antes del ingreso no hay derecho que devengar. Es una entrada normal (los
	// jobs se reejecutan contra fechas pasadas), así que se devuelve un
	// proporcional en cero con la ventana vacía, no un error.
	if fecha.Before(ingreso) {
		return Proporcional{PeriodoDesde: ingreso, Hasta: ingreso, Dias: decimal.Zero}
	}

	// Después del término el derecho dejó de crecer: se calcula hasta esa fecha,
	// no hasta la fecha de consulta.
	if c.FechaTermino != nil {
		if termino := SoloFecha(*c.FechaTermino); fecha.After(termino) {
			fecha = termino
		}
	}

	inicio := UltimoAniversario(ingreso, fecha)

	meses := MesesEntre(inicio, fecha)
	baseDelMes := inicio.AddDate(0, meses, 0)
	dias := DiasEntre(baseDelMes, fecha)
	if dias < 0 {
		// AddDate desborda al mes siguiente cuando el aniversario cae en un fin
		// de mes más largo que el mes destino (un ingreso el 31 de enero da un
		// "31 de febrero" que salta a marzo). Sin este recorte, RRHH leería
		// "1 mes y −2 días" en la pantalla de finiquito.
		dias = 0
	}

	porMeses := diasPorMes.Mul(decimal.NewFromInt(int64(meses)))
	porDias := decimal.NewFromInt(int64(dias)).Div(treinta).Mul(diasPorMes)

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
		remanente, aporta := disponibleDeBolsa(b, fecha)
		if !aporta {
			continue
		}
		disponible = disponible.Add(remanente)
	}

	return Finiquito{
		Proporcional:      proporcional,
		DisponiblePagable: disponible.Round(2),
		Total:             proporcional.Dias.Add(disponible).Round(2),
	}
}
