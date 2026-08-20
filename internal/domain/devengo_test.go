package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// tipoLegal replica la configuración del feriado legal chileno.
func tipoLegal() TipoDeVacacion {
	return TipoDeVacacion{
		Codigo:              "FERIADO_LEGAL",
		PoliticaDevengo:     DevengoAniversarioLegal,
		PoliticaVencimiento: VencimientoNPeriodos,
		PrioridadConsumo:    10,
		UnidadHabil:         true,
		PagableEnFiniquito:  true,
		Parametros: Parametros{
			DiasBase:                        "15",
			ProgresivoActivo:                true,
			ProgresivoUmbralMeses:           120,
			ProgresivoAntiguedadMinimaMeses: 36,
			ProgresivoCadenciaAnios:         3,
			ProgresivoDiasPorTramo:          "1",
			NPeriodos:                       2,
		},
	}
}

// carlos es el colaborador antiguo de la demo: ingreso 2017-04-15 con 24 meses
// de experiencia previa acreditada.
func carlos() Colaborador {
	return Colaborador{
		FechaIngreso:           Fecha(2017, 4, 15),
		MesesExperienciaPrevia: 24,
	}
}

// CASO DE REFERENCIA DEL DOCUMENTO: 9 años con el empleador actual dan 3 días
// progresivos, para un total de 18 días.
func TestDevengo_NueveAniosDanDieciochoDias(t *testing.T) {
	res, hubo := Devengar(tipoLegal(), carlos(), Fecha(2026, 4, 15))

	if !hubo {
		t.Fatal("el 2026-04-15 es aniversario de ingreso: debía devengar")
	}
	if !res.Dias.Equal(decimal.RequireFromString("18")) {
		t.Fatalf("Dias = %s, esperado 18", res.Dias)
	}
	if !res.DiasProgresivo.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("DiasProgresivo = %s, esperado 3", res.DiasProgresivo)
	}
}

func TestDevengo_ProgresivoEnLosBordes(t *testing.T) {
	casos := []struct {
		nombre      string
		fecha       time.Time
		esperado    string
		explicacion string
	}{
		{"séptimo aniversario", Fecha(2024, 4, 15), "15",
			"84 + 24 = 108 meses de experiencia total, bajo el umbral de 120"},
		{"octavo aniversario, justo en el umbral", Fecha(2025, 4, 15), "17",
			"96 + 24 = 120 meses exactos; floor(8/3) = 2 días extra"},
		{"noveno aniversario", Fecha(2026, 4, 15), "18",
			"floor(9/3) = 3 días extra"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res, hubo := Devengar(tipoLegal(), carlos(), c.fecha)
			if !hubo {
				t.Fatalf("debía devengar en %s", c.fecha.Format("2006-01-02"))
			}
			if !res.Dias.Equal(decimal.RequireFromString(c.esperado)) {
				t.Fatalf("Dias = %s, esperado %s (%s)", res.Dias, c.esperado, c.explicacion)
			}
		})
	}
}

func TestDevengo_AntiguedadMinimaBloqueaElProgresivo(t *testing.T) {
	// Alguien con mucha experiencia previa pero recién llegado: supera el umbral
	// de 120 meses totales, pero no los 36 meses continuos exigidos.
	novato := Colaborador{FechaIngreso: Fecha(2024, 4, 15), MesesExperienciaPrevia: 200}

	res, hubo := Devengar(tipoLegal(), novato, Fecha(2026, 4, 15))
	if !hubo {
		t.Fatal("debía devengar")
	}
	if !res.Dias.Equal(decimal.RequireFromString("15")) {
		t.Fatalf("Dias = %s, esperado 15: no cumple los 36 meses continuos", res.Dias)
	}
}

func TestDevengo_SoloEnElAniversario(t *testing.T) {
	if _, hubo := Devengar(tipoLegal(), carlos(), Fecha(2026, 4, 16)); hubo {
		t.Fatal("no debía devengar un día que no es aniversario")
	}
	if _, hubo := Devengar(tipoLegal(), carlos(), Fecha(2017, 4, 15)); hubo {
		t.Fatal("el día de ingreso no devenga")
	}
}

func TestDevengo_PeriodoDelOtorgamiento(t *testing.T) {
	res, _ := Devengar(tipoLegal(), carlos(), Fecha(2026, 4, 15))

	if !res.PeriodoDesde.Equal(Fecha(2025, 4, 15)) {
		t.Fatalf("PeriodoDesde = %s, esperado 2025-04-15", res.PeriodoDesde.Format("2006-01-02"))
	}
	if !res.PeriodoHasta.Equal(Fecha(2026, 4, 14)) {
		t.Fatalf("PeriodoHasta = %s, esperado 2026-04-14", res.PeriodoHasta.Format("2006-01-02"))
	}
}

func TestDevengo_AnioCalendario(t *testing.T) {
	administrativo := TipoDeVacacion{
		Codigo:              "ADMINISTRATIVO",
		PoliticaDevengo:     DevengoAnioCalendario,
		PoliticaVencimiento: VencimientoFinDeAnio,
		Parametros:          Parametros{DiasBase: "6"},
	}
	c := Colaborador{FechaIngreso: Fecha(2020, 3, 1)}

	res, hubo := Devengar(administrativo, c, Fecha(2026, 1, 1))
	if !hubo {
		t.Fatal("el año calendario devenga el 1 de enero")
	}
	if !res.Dias.Equal(decimal.RequireFromString("6")) {
		t.Fatalf("Dias = %s, esperado 6", res.Dias)
	}
	if !res.PeriodoHasta.Equal(Fecha(2026, 12, 31)) {
		t.Fatalf("PeriodoHasta = %s, esperado 2026-12-31", res.PeriodoHasta.Format("2006-01-02"))
	}

	if _, hubo := Devengar(administrativo, c, Fecha(2026, 1, 2)); hubo {
		t.Fatal("el año calendario no devenga el 2 de enero")
	}
}

func TestDevengo_ManualNuncaDevengaSolo(t *testing.T) {
	rendimiento := TipoDeVacacion{
		Codigo:          "RENDIMIENTO",
		PoliticaDevengo: DevengoManual,
		Parametros:      Parametros{DiasBase: "2"},
	}
	if _, hubo := Devengar(rendimiento, carlos(), Fecha(2026, 4, 15)); hubo {
		t.Fatal("un tipo manual nunca devenga automáticamente")
	}
}

func TestDevengo_ProgresivoJustoSobreLaAntiguedadMinima(t *testing.T) {
	// Antigüedad de exactamente 36 meses, el mínimo continuo exigido, con
	// experiencia previa suficiente para cruzar el umbral de 120 meses totales.
	// Es el borde superior de la regla de antigüedad: floor(3/3) = 1 día extra.
	justoEnElBorde := Colaborador{
		FechaIngreso:           Fecha(2023, 4, 15),
		MesesExperienciaPrevia: 84,
	}

	res, hubo := Devengar(tipoLegal(), justoEnElBorde, Fecha(2026, 4, 15))
	if !hubo {
		t.Fatal("debía devengar")
	}
	if !res.Dias.Equal(decimal.RequireFromString("16")) {
		t.Fatalf("Dias = %s, esperado 16: 36 meses de antigüedad exactos ya habilitan un tramo", res.Dias)
	}
}

// Recorre día a día tres años completos: un ingreso del 29 de febrero debe
// devengar exactamente una vez al año, no cero veces.
func TestDevengo_IngresoEn29DeFebreroDevengaUnaVezPorAnio(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2024, 2, 29)}

	var fechas []time.Time
	for d := Fecha(2025, 1, 1); !d.After(Fecha(2027, 12, 31)); d = d.AddDate(0, 0, 1) {
		if _, hubo := Devengar(tipoLegal(), c, d); hubo {
			fechas = append(fechas, d)
		}
	}

	if len(fechas) != 3 {
		t.Fatalf("esperaba 3 devengos entre 2025 y 2027, dio %d: %v", len(fechas), fechas)
	}
	esperadas := []time.Time{Fecha(2025, 2, 28), Fecha(2026, 2, 28), Fecha(2027, 2, 28)}
	for i, e := range esperadas {
		if !fechas[i].Equal(e) {
			t.Fatalf("devengo %d = %s, esperado %s",
				i, fechas[i].Format("2006-01-02"), e.Format("2006-01-02"))
		}
	}
}

// Los jobs se reejecutan contra fechas arbitrarias del pasado: evaluar a alguien
// fuera de su ventana de empleo es una entrada rutinaria, no un error.
func TestDevengo_RespetaLaVentanaDeEmpleo(t *testing.T) {
	termino := Fecha(2026, 4, 15)
	desvinculado := Colaborador{FechaIngreso: Fecha(2017, 4, 15), FechaTermino: &termino}

	if _, hubo := Devengar(tipoLegal(), desvinculado, Fecha(2026, 4, 15)); hubo {
		t.Fatal("no debe devengar el día del término: la relación ya terminó")
	}
	if _, hubo := Devengar(tipoLegal(), desvinculado, Fecha(2027, 4, 15)); hubo {
		t.Fatal("no debe devengar un aniversario posterior al término")
	}

	sigueVigente := Fecha(2026, 4, 16)
	activo := Colaborador{FechaIngreso: Fecha(2017, 4, 15), FechaTermino: &sigueVigente}
	if _, hubo := Devengar(tipoLegal(), activo, Fecha(2026, 4, 15)); !hubo {
		t.Fatal("el día anterior al término todavía devenga")
	}
}

func TestDevengo_NoDevengaAntesDelIngreso(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2026, 6, 1)}

	if _, hubo := Devengar(tipoLegal(), c, Fecha(2026, 1, 15)); hubo {
		t.Fatal("no debe devengar antes de la fecha de ingreso")
	}

	administrativo := TipoDeVacacion{
		PoliticaDevengo:     DevengoAnioCalendario,
		PoliticaVencimiento: VencimientoFinDeAnio,
		Parametros:          Parametros{DiasBase: "6"},
	}
	if _, hubo := Devengar(administrativo, c, Fecha(2026, 1, 1)); hubo {
		t.Fatal("el año calendario tampoco devenga antes del ingreso")
	}
}

// El período de un otorgamiento debe pegar exactamente con el del otorgamiento
// anterior: sin recorte de fin de mes, el aniversario se calculaba recortado
// pero el PeriodoDesde derivado de él no, y quedaba un hueco de días que no
// pertenecían a ningún período.
func TestDevengo_PeriodosContiguosEnAniversarioBisiesto(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2024, 2, 29)}

	previo, hubo := Devengar(tipoLegal(), c, Fecha(2027, 2, 28))
	if !hubo {
		t.Fatal("2027-02-28 es el aniversario recortado: debía devengar")
	}
	siguiente, hubo := Devengar(tipoLegal(), c, Fecha(2028, 2, 29))
	if !hubo {
		t.Fatal("2028-02-29 es el aniversario bisiesto: debía devengar")
	}

	if !siguiente.PeriodoDesde.Equal(Fecha(2027, 2, 28)) {
		t.Fatalf("PeriodoDesde = %s, esperado 2027-02-28",
			siguiente.PeriodoDesde.Format("2006-01-02"))
	}

	// La contigüidad es el defecto real, no la fecha suelta.
	diaSiguiente := previo.PeriodoHasta.AddDate(0, 0, 1)
	if !diaSiguiente.Equal(siguiente.PeriodoDesde) {
		t.Fatalf("hueco entre períodos: el previo termina el %s y el siguiente empieza el %s",
			previo.PeriodoHasta.Format("2006-01-02"), siguiente.PeriodoDesde.Format("2006-01-02"))
	}
}
