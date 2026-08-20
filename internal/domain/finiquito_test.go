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
