package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestBolsaRemanente(t *testing.T) {
	id := uuid.New()
	b := Bolsa{
		Otorgamiento: Otorgamiento{ID: id, DiasOtorgados: decimal.RequireFromString("18")},
		Movimientos: []Movimiento{
			{OtorgamientoID: id, Clase: ClaseAccrual, Cantidad: decimal.RequireFromString("18")},
			{OtorgamientoID: id, Clase: ClaseConsumption, Cantidad: decimal.RequireFromString("-5")},
			{OtorgamientoID: id, Clase: ClaseConsumption, Cantidad: decimal.RequireFromString("-3")},
		},
	}

	if !b.Remanente().Equal(decimal.RequireFromString("10")) {
		t.Fatalf("Remanente = %s, esperado 10", b.Remanente())
	}
}

func TestBolsaVigente(t *testing.T) {
	vence := Fecha(2026, 4, 15)
	b := Bolsa{Otorgamiento: Otorgamiento{VenceEl: &vence}}

	if !b.VigenteAl(Fecha(2026, 4, 14)) {
		t.Fatal("un día antes del vencimiento la bolsa sigue vigente")
	}
	if b.VigenteAl(Fecha(2026, 4, 15)) {
		t.Fatal("el día del vencimiento la bolsa ya no sirve: vence_el es exclusivo")
	}

	sinVencimiento := Bolsa{Otorgamiento: Otorgamiento{VenceEl: nil}}
	if !sinVencimiento.VigenteAl(Fecha(2099, 1, 1)) {
		t.Fatal("una bolsa sin vencimiento siempre está vigente")
	}
}

func TestClaveIdempotencia(t *testing.T) {
	colaborador := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tipo := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	got := ClaveIdempotencia(ClaseAccrual, colaborador, tipo, Fecha(2026, 4, 15))
	esperado := "ACCRUAL:11111111-1111-1111-1111-111111111111:22222222-2222-2222-2222-222222222222:2026-04-15"

	if got != esperado {
		t.Fatalf("ClaveIdempotencia = %q, esperado %q", got, esperado)
	}
}

// zonaDePrueba imita una zona con offset negativo sin depender de la tzdata de
// la máquina donde corren las pruebas.
func zonaDePrueba() *time.Location { return time.FixedZone("test", -4*3600) }

// Un vence_el leído de la base con zona (no UTC medianoche) hacía que una bolsa
// ya vencida siguiera reportándose como usable: la comparación era contra el
// instante crudo, no contra la fecha.
func TestBolsaVigente_NoDependeDeLaZonaDelVencimiento(t *testing.T) {
	venceUTC := Fecha(2026, 4, 15)
	venceConZona := time.Date(2026, 4, 15, 0, 0, 0, 0, zonaDePrueba())

	conZona := Bolsa{Otorgamiento: Otorgamiento{VenceEl: &venceConZona}}
	enUTC := Bolsa{Otorgamiento: Otorgamiento{VenceEl: &venceUTC}}

	for _, fecha := range []time.Time{Fecha(2026, 4, 14), Fecha(2026, 4, 15), Fecha(2026, 4, 16)} {
		if got, esperado := conZona.VigenteAl(fecha), enUTC.VigenteAl(fecha); got != esperado {
			t.Fatalf("VigenteAl(%s) con zona = %v, en UTC = %v: deben coincidir",
				fecha.Format("2006-01-02"), got, esperado)
		}
	}

	if conZona.VigenteAl(Fecha(2026, 4, 15)) {
		t.Fatal("el día del vencimiento la bolsa ya no sirve, venga vence_el con zona o sin ella")
	}
}

// La clave de idempotencia es la única defensa contra otorgar dos veces al
// reejecutar un job: no puede depender de la zona que traiga el llamador.
func TestClavesIdempotencia_NoDependenDeLaZona(t *testing.T) {
	colaborador := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tipo := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	otorgamiento := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	enUTC := Fecha(2026, 4, 15)
	conZona := time.Date(2026, 4, 15, 0, 0, 0, 0, zonaDePrueba())
	conHora := time.Date(2026, 4, 15, 23, 30, 0, 0, zonaDePrueba())

	for _, fecha := range []time.Time{conZona, conHora} {
		if got, esperado := ClaveIdempotencia(ClaseAccrual, colaborador, tipo, fecha),
			ClaveIdempotencia(ClaseAccrual, colaborador, tipo, enUTC); got != esperado {
			t.Fatalf("ClaveIdempotencia = %q, esperado %q", got, esperado)
		}
		if got, esperado := ClaveIdempotenciaBolsa(ClaseExpiration, otorgamiento, fecha),
			ClaveIdempotenciaBolsa(ClaseExpiration, otorgamiento, enUTC); got != esperado {
			t.Fatalf("ClaveIdempotenciaBolsa = %q, esperado %q", got, esperado)
		}
	}
}

// El signo es lo que hace funcionar el ledger: una bolsa con ACCRUAL +15 y
// CONSUMPTION +5 reporta 20 en vez de 10, y un movimiento equivocado no se
// puede corregir reejecutando nada.
func TestMovimientoValidar(t *testing.T) {
	casos := []struct {
		nombre      string
		clase       ClaseMovimiento
		cantidad    string
		esperaError bool
	}{
		{"accrual positivo", ClaseAccrual, "15", false},
		{"accrual negativo", ClaseAccrual, "-15", true},
		{"saldo inicial positivo", ClaseOpening, "10", false},
		{"saldo inicial negativo", ClaseOpening, "-10", true},
		{"ajuste positivo", ClaseAdjustment, "2", false},
		{"ajuste negativo", ClaseAdjustment, "-2", true},
		{"consumo negativo", ClaseConsumption, "-5", false},
		{"consumo positivo", ClaseConsumption, "5", true},
		{"vencimiento negativo", ClaseExpiration, "-3", false},
		{"vencimiento positivo", ClaseExpiration, "3", true},
		{"pago de finiquito negativo", ClasePayout, "-8", false},
		{"pago de finiquito positivo", ClasePayout, "8", true},
		{"reversal positivo: refleja lo que revierte", ClaseReversal, "5", false},
		{"reversal negativo: también", ClaseReversal, "-5", false},
		{"cero en accrual", ClaseAccrual, "0", true},
		{"cero en consumo", ClaseConsumption, "0", true},
		{"cero en reversal", ClaseReversal, "0", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			m := Movimiento{Clase: c.clase, Cantidad: decimal.RequireFromString(c.cantidad)}
			err := m.Validar()
			if c.esperaError && err == nil {
				t.Fatalf("%s con cantidad %s debía ser inválido", c.clase, c.cantidad)
			}
			if !c.esperaError && err != nil {
				t.Fatalf("%s con cantidad %s debía ser válido, dio: %v", c.clase, c.cantidad, err)
			}
		})
	}
}

func TestMovimientoValidar_ClaseDesconocida(t *testing.T) {
	m := Movimiento{Clase: ClaseMovimiento("INVENTADA"), Cantidad: decimal.RequireFromString("1")}
	if m.Validar() == nil {
		t.Fatal("una clase desconocida debe ser inválida")
	}
}
