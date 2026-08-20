package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func bolsa(vence *time.Time, remanente string, prioridad int) Bolsa {
	id := uuid.New()
	return Bolsa{
		Otorgamiento: Otorgamiento{ID: id, VenceEl: vence},
		Prioridad:    prioridad,
		Movimientos: []Movimiento{
			{OtorgamientoID: id, Clase: ClaseAccrual, Cantidad: decimal.RequireFromString(remanente)},
		},
	}
}

func TestAsignarConsumo_RepartEntreVariasBolsas(t *testing.T) {
	pronto := Fecha(2026, 12, 31)
	tarde := Fecha(2027, 4, 15)

	bolsas := []Bolsa{
		bolsa(&tarde, "17", 10), // deliberadamente primero en la lista
		bolsa(&pronto, "3", 10), // pero vence antes: debe consumirse primero
	}

	asignaciones, err := AsignarConsumo(bolsas, decimal.RequireFromString("8"), Fecha(2026, 9, 1))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(asignaciones) != 2 {
		t.Fatalf("esperaba 2 asignaciones, dio %d", len(asignaciones))
	}
	if asignaciones[0].OtorgamientoID != bolsas[1].Otorgamiento.ID {
		t.Fatal("la primera asignación debe salir de la bolsa que vence antes")
	}
	if !asignaciones[0].Dias.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("primera asignación = %s, esperado 3", asignaciones[0].Dias)
	}
	if !asignaciones[1].Dias.Equal(decimal.RequireFromString("5")) {
		t.Fatalf("segunda asignación = %s, esperado 5", asignaciones[1].Dias)
	}
}

func TestAsignarConsumo_DesempataPorPrioridad(t *testing.T) {
	misma := Fecha(2027, 1, 1)
	baja := bolsa(&misma, "5", 20)
	alta := bolsa(&misma, "5", 10) // menor número = se consume antes

	asignaciones, err := AsignarConsumo([]Bolsa{baja, alta}, decimal.RequireFromString("2"), Fecha(2026, 9, 1))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if asignaciones[0].OtorgamientoID != alta.Otorgamiento.ID {
		t.Fatal("a igual vencimiento debe consumirse primero la de menor prioridad numérica")
	}
}

func TestAsignarConsumo_LasBolsasSinVencimientoVanAlFinal(t *testing.T) {
	vence := Fecha(2027, 1, 1)
	conVencimiento := bolsa(&vence, "4", 10)
	sinVencimiento := bolsa(nil, "10", 10)

	asignaciones, err := AsignarConsumo(
		[]Bolsa{sinVencimiento, conVencimiento}, decimal.RequireFromString("6"), Fecha(2026, 9, 1))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if asignaciones[0].OtorgamientoID != conVencimiento.Otorgamiento.ID {
		t.Fatal("la bolsa con vencimiento debe consumirse antes que la que no vence")
	}
}

func TestAsignarConsumo_IgnoraBolsasVencidasYVacias(t *testing.T) {
	vencida := Fecha(2026, 1, 1)
	viva := Fecha(2027, 1, 1)

	asignaciones, err := AsignarConsumo([]Bolsa{
		bolsa(&vencida, "10", 10), // ya venció al 2026-09-01
		bolsa(&viva, "0", 10),     // vigente pero sin remanente
		bolsa(&viva, "4", 10),
	}, decimal.RequireFromString("4"), Fecha(2026, 9, 1))

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(asignaciones) != 1 {
		t.Fatalf("esperaba 1 asignación, dio %d", len(asignaciones))
	}
}

func TestAsignarConsumo_SaldoInsuficiente(t *testing.T) {
	viva := Fecha(2027, 1, 1)

	_, err := AsignarConsumo([]Bolsa{bolsa(&viva, "3", 10)},
		decimal.RequireFromString("5"), Fecha(2026, 9, 1))

	if !errors.Is(err, ErrSaldoInsuficiente) {
		t.Fatalf("esperaba ErrSaldoInsuficiente, dio %v", err)
	}
}
