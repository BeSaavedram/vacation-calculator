package domain

import (
	"testing"
	"time"
)

func TestMesesEntre(t *testing.T) {
	casos := []struct {
		nombre       string
		desde, hasta time.Time
		esperado     int
	}{
		{"nueve años exactos", Fecha(2017, 4, 15), Fecha(2026, 4, 15), 108},
		{"un día antes del aniversario", Fecha(2017, 4, 15), Fecha(2026, 4, 14), 107},
		{"mismo mes", Fecha(2026, 1, 1), Fecha(2026, 1, 31), 0},
		{"ocho meses y días", Fecha(2026, 1, 1), Fecha(2026, 9, 25), 8},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := MesesEntre(c.desde, c.hasta); got != c.esperado {
				t.Fatalf("MesesEntre(%s, %s) = %d, esperado %d",
					c.desde.Format("2006-01-02"), c.hasta.Format("2006-01-02"), got, c.esperado)
			}
		})
	}
}

func TestUltimoAniversario(t *testing.T) {
	casos := []struct {
		nombre         string
		ingreso, fecha time.Time
		esperado       time.Time
	}{
		{"antes del aniversario del año", Fecha(2018, 4, 15), Fecha(2026, 2, 1), Fecha(2025, 4, 15)},
		{"después del aniversario", Fecha(2018, 4, 15), Fecha(2026, 9, 25), Fecha(2026, 4, 15)},
		{"justo en el aniversario", Fecha(2018, 4, 15), Fecha(2026, 4, 15), Fecha(2026, 4, 15)},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := UltimoAniversario(c.ingreso, c.fecha); !got.Equal(c.esperado) {
				t.Fatalf("UltimoAniversario = %s, esperado %s",
					got.Format("2006-01-02"), c.esperado.Format("2006-01-02"))
			}
		})
	}
}

func TestEsAniversario(t *testing.T) {
	ingreso := Fecha(2017, 4, 15)
	if !EsAniversario(ingreso, Fecha(2026, 4, 15)) {
		t.Fatal("2026-04-15 debería ser aniversario de un ingreso 2017-04-15")
	}
	if EsAniversario(ingreso, Fecha(2026, 4, 16)) {
		t.Fatal("2026-04-16 no debería ser aniversario")
	}
	if EsAniversario(ingreso, ingreso) {
		t.Fatal("el día de ingreso no es un aniversario")
	}
}
