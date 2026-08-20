package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CASO DE REFERENCIA DEL DOCUMENTO: 8 meses completos y 24 días del período en
// curso dan 8 × 1,25 + (24/30) × 1,25 = 11,00 días.
func TestProporcional_CasoDeReferencia(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 1, 1)}

	p := CalcularProporcional(c, Fecha(2026, 9, 25))

	if p.MesesCompletos != 8 {
		t.Fatalf("MesesCompletos = %d, esperado 8", p.MesesCompletos)
	}
	if p.DiasRestantes != 24 {
		t.Fatalf("DiasRestantes = %d, esperado 24", p.DiasRestantes)
	}
	if !p.Dias.Equal(decimal.RequireFromString("11")) {
		t.Fatalf("Dias = %s, esperado 11.00", p.Dias)
	}
}

func TestProporcional_JustoEnElAniversarioEsCero(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 4, 15)}

	p := CalcularProporcional(c, Fecha(2026, 4, 15))

	if !p.Dias.Equal(decimal.Zero) {
		t.Fatalf("Dias = %s, esperado 0: el período en curso recién empieza", p.Dias)
	}
}

func TestProporcional_UnMesExacto(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 1, 1)}

	p := CalcularProporcional(c, Fecha(2026, 2, 1))

	if !p.Dias.Equal(decimal.RequireFromString("1.25")) {
		t.Fatalf("Dias = %s, esperado 1.25", p.Dias)
	}
}

func TestCalcularFiniquito_SumaProporcionalYDisponiblePagable(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 1, 1)}
	tipoPagable := uuid.New()
	tipoNoPagable := uuid.New()
	vence := Fecha(2028, 1, 1)

	bolsas := []Bolsa{
		bolsaDeTipo(tipoPagable, &vence, "10"),
		bolsaDeTipo(tipoNoPagable, &vence, "6"), // administrativo: no se paga
	}
	pagables := map[uuid.UUID]bool{tipoPagable: true, tipoNoPagable: false}

	f := CalcularFiniquito(c, bolsas, pagables, Fecha(2026, 9, 25))

	if !f.Proporcional.Dias.Equal(decimal.RequireFromString("11")) {
		t.Fatalf("Proporcional = %s, esperado 11.00", f.Proporcional.Dias)
	}
	if !f.DisponiblePagable.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("DisponiblePagable = %s, esperado 10: el no pagable no cuenta", f.DisponiblePagable)
	}
	if !f.Total.Equal(decimal.RequireFromString("21")) {
		t.Fatalf("Total = %s, esperado 21.00", f.Total)
	}
}

func bolsaDeTipo(tipoID uuid.UUID, vence *time.Time, remanente string) Bolsa {
	id := uuid.New()
	return Bolsa{
		Otorgamiento: Otorgamiento{ID: id, TipoID: tipoID, VenceEl: vence},
		Movimientos: []Movimiento{
			{OtorgamientoID: id, Clase: ClaseAccrual, Cantidad: decimal.RequireFromString(remanente)},
		},
	}
}

// Una bolsa con remanente negativo es una anomalía de datos. Saldo y finiquito
// deben tratarla igual: si dieran números distintos, RRHH vería dos cifras para
// la misma persona el mismo día.
func TestDisponible_SaldoYFiniquitoCoinciden(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 1, 1)}
	tipoID := uuid.New()
	vence := Fecha(2028, 1, 1)

	sana := bolsaDeTipo(tipoID, &vence, "5")
	anomala := bolsaConMovimientos(tipoID, &vence, "3", "-5") // remanente -2

	bolsas := []Bolsa{sana, anomala}
	tipos := map[uuid.UUID]TipoDeVacacion{tipoID: {ID: tipoID, Codigo: "FERIADO_LEGAL"}}

	saldo := ProyectarSaldo(c.ID, bolsas, tipos, Fecha(2026, 9, 25))
	finiquito := CalcularFiniquito(c, bolsas, map[uuid.UUID]bool{tipoID: true}, Fecha(2026, 9, 25))

	if !saldo.Total().Equal(finiquito.DisponiblePagable) {
		t.Fatalf("Saldo.Total() = %s pero Finiquito.DisponiblePagable = %s: deben ser el mismo número",
			saldo.Total(), finiquito.DisponiblePagable)
	}
	if !saldo.Total().Equal(decimal.RequireFromString("5")) {
		t.Fatalf("disponible = %s, esperado 5: la bolsa anómala se descarta, no se netea",
			saldo.Total())
	}
}

// bolsaConMovimientos arma una bolsa con un otorgamiento y su consumo, para
// poder construir remanentes negativos.
func bolsaConMovimientos(tipoID uuid.UUID, vence *time.Time, otorgado, consumido string) Bolsa {
	id := uuid.New()
	return Bolsa{
		Otorgamiento: Otorgamiento{ID: id, TipoID: tipoID, VenceEl: vence},
		Movimientos: []Movimiento{
			{OtorgamientoID: id, Clase: ClaseAccrual, Cantidad: decimal.RequireFromString(otorgado)},
			{OtorgamientoID: id, Clase: ClaseConsumption, Cantidad: decimal.RequireFromString(consumido)},
		},
	}
}

// Nueve días devengados para alguien que todavía no entraba: el job se reejecuta
// contra fechas pasadas, así que esta entrada es rutinaria.
func TestProporcional_AntesDelIngresoEsCero(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2026, 6, 1)}

	p := CalcularProporcional(c, Fecha(2026, 1, 15))

	if !p.Dias.Equal(decimal.Zero) {
		t.Fatalf("Dias = %s, esperado 0: el colaborador aún no ingresaba", p.Dias)
	}
	if p.MesesCompletos != 0 || p.DiasRestantes != 0 {
		t.Fatalf("MesesCompletos = %d y DiasRestantes = %d, esperado 0 y 0",
			p.MesesCompletos, p.DiasRestantes)
	}
	if !p.PeriodoDesde.Equal(Fecha(2026, 6, 1)) || !p.Hasta.Equal(Fecha(2026, 6, 1)) {
		t.Fatalf("período = %s..%s, esperado la ventana vacía en la fecha de ingreso",
			p.PeriodoDesde.Format("2006-01-02"), p.Hasta.Format("2006-01-02"))
	}
}

func TestProporcional_JustoEnElIngresoEsCero(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2026, 6, 1)}

	if p := CalcularProporcional(c, Fecha(2026, 6, 1)); !p.Dias.Equal(decimal.Zero) {
		t.Fatalf("Dias = %s, esperado 0 el día mismo del ingreso", p.Dias)
	}
}

// Después del término el derecho dejó de crecer: se calcula hasta esa fecha.
func TestProporcional_SeDetieneEnElTermino(t *testing.T) {
	termino := Fecha(2026, 9, 25)
	c := Colaborador{FechaIngreso: Fecha(2018, 1, 1), FechaTermino: &termino}

	enElTermino := CalcularProporcional(c, Fecha(2026, 9, 25))
	muyDespues := CalcularProporcional(c, Fecha(2026, 12, 31))

	if !muyDespues.Dias.Equal(enElTermino.Dias) {
		t.Fatalf("Dias después del término = %s, esperado %s: el derecho dejó de crecer",
			muyDespues.Dias, enElTermino.Dias)
	}
	if !muyDespues.Dias.Equal(decimal.RequireFromString("11")) {
		t.Fatalf("Dias = %s, esperado 11.00", muyDespues.Dias)
	}
	if !muyDespues.Hasta.Equal(termino) {
		t.Fatalf("Hasta = %s, esperado el término %s",
			muyDespues.Hasta.Format("2006-01-02"), termino.Format("2006-01-02"))
	}
}

// FIX 7: AddDate desbordaba el fin de mes y RRHH veía "1 mes y −2 días".
func TestProporcional_DiasRestantesNuncaEsNegativo(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2020, 1, 31)}

	p := CalcularProporcional(c, Fecha(2026, 3, 1))

	if p.DiasRestantes < 0 {
		t.Fatalf("DiasRestantes = %d: nunca debe ser negativo", p.DiasRestantes)
	}
	if p.MesesCompletos != 1 {
		t.Fatalf("MesesCompletos = %d, esperado 1", p.MesesCompletos)
	}
}
