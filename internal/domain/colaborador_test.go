package domain

import "testing"

func TestColaboradorMesesAntiguedad(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2017, 4, 15)}
	if got := c.MesesAntiguedadAl(Fecha(2026, 4, 15)); got != 108 {
		t.Fatalf("MesesAntiguedadAl = %d, esperado 108", got)
	}
}

func TestColaboradorMesesExperienciaTotal(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2017, 4, 15), MesesExperienciaPrevia: 24}
	// 96 meses con el empleador + 24 previos = 120 justo en el umbral legal
	if got := c.MesesExperienciaTotalAl(Fecha(2025, 4, 15)); got != 120 {
		t.Fatalf("MesesExperienciaTotalAl = %d, esperado 120", got)
	}
}

func TestColaboradorAniosConEmpleador(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2017, 4, 15)}
	if got := c.AniosConEmpleadorAl(Fecha(2026, 4, 15)); got != 9 {
		t.Fatalf("AniosConEmpleadorAl = %d, esperado 9", got)
	}
	if got := c.AniosConEmpleadorAl(Fecha(2026, 4, 14)); got != 8 {
		t.Fatalf("un día antes del aniversario debe dar 8, dio %d", got)
	}
}
