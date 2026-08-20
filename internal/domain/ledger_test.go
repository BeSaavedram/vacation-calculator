package domain

import (
	"testing"

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
