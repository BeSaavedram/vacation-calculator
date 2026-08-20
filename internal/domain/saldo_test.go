package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestProyectarSaldo_SumaSoloBolsasVigentes(t *testing.T) {
	tipoID := uuid.New()
	vencida := Fecha(2026, 1, 1)
	viva := Fecha(2027, 4, 15)

	bolsas := []Bolsa{
		bolsaDeTipo(tipoID, &vencida, "15"), // ya venció
		bolsaDeTipo(tipoID, &viva, "18"),
	}
	tipos := map[uuid.UUID]TipoDeVacacion{
		tipoID: {ID: tipoID, Codigo: "FERIADO_LEGAL", Nombre: "Feriado legal"},
	}

	saldo := ProyectarSaldo(bolsas, tipos, Fecha(2026, 9, 1))

	if len(saldo.PorTipo) != 1 {
		t.Fatalf("esperaba 1 tipo, dio %d", len(saldo.PorTipo))
	}
	if !saldo.PorTipo[0].Disponible.Equal(decimal.RequireFromString("18")) {
		t.Fatalf("Disponible = %s, esperado 18", saldo.PorTipo[0].Disponible)
	}
}

func TestProyectarSaldo_MarcaLoQueEstaPorVencer(t *testing.T) {
	tipoID := uuid.New()
	pronto := Fecha(2026, 10, 15) // dentro de 44 días
	lejos := Fecha(2028, 1, 1)

	bolsas := []Bolsa{
		bolsaDeTipo(tipoID, &pronto, "3"),
		bolsaDeTipo(tipoID, &lejos, "18"),
	}
	tipos := map[uuid.UUID]TipoDeVacacion{tipoID: {ID: tipoID, Codigo: "FERIADO_LEGAL"}}

	saldo := ProyectarSaldo(bolsas, tipos, Fecha(2026, 9, 1))

	if len(saldo.PorTipo[0].PorVencer) != 1 {
		t.Fatalf("esperaba 1 bolsa por vencer, dio %d", len(saldo.PorTipo[0].PorVencer))
	}
	if !saldo.PorTipo[0].PorVencer[0].Dias.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("PorVencer = %s, esperado 3", saldo.PorTipo[0].PorVencer[0].Dias)
	}
}

func TestProyectarSaldo_AgrupaPorTipo(t *testing.T) {
	legal, administrativo := uuid.New(), uuid.New()
	viva := Fecha(2028, 1, 1)

	bolsas := []Bolsa{
		bolsaDeTipo(legal, &viva, "18"),
		bolsaDeTipo(administrativo, &viva, "6"),
	}
	tipos := map[uuid.UUID]TipoDeVacacion{
		legal:          {ID: legal, Codigo: "FERIADO_LEGAL", PrioridadConsumo: 10},
		administrativo: {ID: administrativo, Codigo: "ADMINISTRATIVO", PrioridadConsumo: 20},
	}

	saldo := ProyectarSaldo(bolsas, tipos, Fecha(2026, 9, 1))

	if len(saldo.PorTipo) != 2 {
		t.Fatalf("esperaba 2 tipos, dio %d", len(saldo.PorTipo))
	}
	// Orden estable por prioridad de consumo
	if saldo.PorTipo[0].TipoCodigo != "FERIADO_LEGAL" {
		t.Fatalf("primer tipo = %s, esperado FERIADO_LEGAL", saldo.PorTipo[0].TipoCodigo)
	}
	if !saldo.Total().Equal(decimal.RequireFromString("24")) {
		t.Fatalf("Total = %s, esperado 24", saldo.Total())
	}
}
