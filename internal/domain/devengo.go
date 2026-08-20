package domain

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ResultadoDevengo es lo que produce el motor cuando corresponde otorgar días.
// Separa base y progresivo porque RRHH necesita poder explicar de dónde salió
// cada día del otorgamiento.
type ResultadoDevengo struct {
	Dias           decimal.Decimal
	DiasBase       decimal.Decimal
	DiasProgresivo decimal.Decimal
	PeriodoDesde   time.Time
	PeriodoHasta   time.Time
	DevengadoEl    time.Time
	Detalle        string
}

// Devengar evalúa si al colaborador le corresponde un otorgamiento de este tipo
// en la fecha dada. El segundo valor indica si hubo devengo.
//
// La fecha entra como parámetro y nunca se lee del reloj: eso es lo que permite
// reejecutar el job con fecha objetivo para recuperar días no procesados.
func Devengar(tipo TipoDeVacacion, c Colaborador, fecha time.Time) (ResultadoDevengo, bool) {
	fecha = SoloFecha(fecha)

	switch tipo.PoliticaDevengo {
	case DevengoAniversarioLegal:
		return devengarAniversario(tipo, c, fecha)
	case DevengoAnioCalendario:
		return devengarAnioCalendario(tipo, c, fecha)
	case DevengoManual:
		// Los tipos manuales solo se otorgan por acción explícita de RRHH.
		return ResultadoDevengo{}, false
	default:
		return ResultadoDevengo{}, false
	}
}

func devengarAniversario(tipo TipoDeVacacion, c Colaborador, fecha time.Time) (ResultadoDevengo, bool) {
	if !EsAniversario(c.FechaIngreso, fecha) {
		return ResultadoDevengo{}, false
	}

	base := tipo.Parametros.DiasBaseDecimal()
	progresivo := DiasProgresivos(tipo.Parametros, c, fecha)

	return ResultadoDevengo{
		Dias:           base.Add(progresivo),
		DiasBase:       base,
		DiasProgresivo: progresivo,
		PeriodoDesde:   fecha.AddDate(-1, 0, 0),
		PeriodoHasta:   fecha.AddDate(0, 0, -1),
		DevengadoEl:    fecha,
		Detalle: fmt.Sprintf("aniversario %d: %s días base + %s progresivos",
			c.AniosConEmpleadorAl(fecha), base, progresivo),
	}, true
}

func devengarAnioCalendario(tipo TipoDeVacacion, c Colaborador, fecha time.Time) (ResultadoDevengo, bool) {
	if fecha.Month() != time.January || fecha.Day() != 1 {
		return ResultadoDevengo{}, false
	}
	if fecha.Before(SoloFecha(c.FechaIngreso)) {
		return ResultadoDevengo{}, false
	}

	base := tipo.Parametros.DiasBaseDecimal()
	return ResultadoDevengo{
		Dias:           base,
		DiasBase:       base,
		DiasProgresivo: decimal.Zero,
		PeriodoDesde:   Fecha(fecha.Year(), time.January, 1),
		PeriodoHasta:   Fecha(fecha.Year(), time.December, 31),
		DevengadoEl:    fecha,
		Detalle:        fmt.Sprintf("año calendario %d: %s días", fecha.Year(), base),
	}, true
}

// DiasProgresivos aplica la regla de feriado progresivo. Umbral, antigüedad
// mínima, cadencia y días por tramo son parámetros de la política: cambiar la
// norma no exige tocar este código.
func DiasProgresivos(p Parametros, c Colaborador, fecha time.Time) decimal.Decimal {
	if !p.ProgresivoActivo {
		return decimal.Zero
	}
	if c.MesesExperienciaTotalAl(fecha) < p.ProgresivoUmbralMeses {
		return decimal.Zero
	}
	if c.MesesAntiguedadAl(fecha) < p.ProgresivoAntiguedadMinimaMeses {
		return decimal.Zero
	}
	if p.ProgresivoCadenciaAnios <= 0 {
		return decimal.Zero
	}

	tramos := c.AniosConEmpleadorAl(fecha) / p.ProgresivoCadenciaAnios
	return p.DiasPorTramoDecimal().Mul(decimal.NewFromInt(int64(tramos)))
}
