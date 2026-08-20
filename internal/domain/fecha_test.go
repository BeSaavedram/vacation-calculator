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

// El 29 de febrero es el borde que ninguna prueba cubría: la convención del
// dominio es que el aniversario se ajusta al último día del mes cuando el mes
// del año destino es más corto que el día de ingreso.
func TestEsAniversario_FinDeMesYAnioBisiesto(t *testing.T) {
	casos := []struct {
		nombre         string
		ingreso, fecha time.Time
		esperado       bool
	}{
		{"ingreso 29-feb, año común: cumple el 28", Fecha(2024, 2, 29), Fecha(2025, 2, 28), true},
		{"ingreso 29-feb, año común: el 1 de marzo no", Fecha(2024, 2, 29), Fecha(2025, 3, 1), false},
		{"ingreso 29-feb, año bisiesto: cumple el 29", Fecha(2024, 2, 29), Fecha(2028, 2, 29), true},
		{"ingreso 29-feb, año bisiesto: el 28 no", Fecha(2024, 2, 29), Fecha(2028, 2, 28), false},
		{"ingreso 31-ene: enero siempre tiene 31", Fecha(2020, 1, 31), Fecha(2026, 1, 31), true},
		{"ingreso 31-ene: el 30 no", Fecha(2020, 1, 31), Fecha(2026, 1, 30), false},
		{"ingreso 31-mar: marzo siempre tiene 31", Fecha(2020, 3, 31), Fecha(2026, 3, 31), true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := EsAniversario(c.ingreso, c.fecha); got != c.esperado {
				t.Fatalf("EsAniversario(%s, %s) = %v, esperado %v",
					c.ingreso.Format("2006-01-02"), c.fecha.Format("2006-01-02"), got, c.esperado)
			}
		})
	}
}

// Un ingreso del 29 de febrero debe tener exactamente UN aniversario por año,
// ni cero (el bug) ni dos.
func TestEsAniversario_UnSoloAniversarioPorAnio(t *testing.T) {
	ingreso := Fecha(2024, 2, 29)
	casos := []struct {
		anio     int
		esperado time.Time
	}{
		{2025, Fecha(2025, 2, 28)},
		{2026, Fecha(2026, 2, 28)},
		{2027, Fecha(2027, 2, 28)},
		{2028, Fecha(2028, 2, 29)},
	}
	for _, c := range casos {
		t.Run(c.esperado.Format("2006"), func(t *testing.T) {
			var encontrados []time.Time
			for d := Fecha(c.anio, 1, 1); d.Year() == c.anio; d = d.AddDate(0, 0, 1) {
				if EsAniversario(ingreso, d) {
					encontrados = append(encontrados, d)
				}
			}
			if len(encontrados) != 1 {
				t.Fatalf("esperaba 1 aniversario en %d, encontró %d: %v", c.anio, len(encontrados), encontrados)
			}
			if !encontrados[0].Equal(c.esperado) {
				t.Fatalf("aniversario = %s, esperado %s",
					encontrados[0].Format("2006-01-02"), c.esperado.Format("2006-01-02"))
			}
		})
	}
}

func TestUltimoAniversario_FinDeMesYAnioBisiesto(t *testing.T) {
	casos := []struct {
		nombre         string
		ingreso, fecha time.Time
		esperado       time.Time
	}{
		{"29-feb visto en junio de un año común", Fecha(2024, 2, 29), Fecha(2025, 6, 1), Fecha(2025, 2, 28)},
		{"29-feb un día antes del aniversario ajustado", Fecha(2024, 2, 29), Fecha(2025, 2, 27), Fecha(2024, 2, 29)},
		{"29-feb justo en el aniversario ajustado", Fecha(2024, 2, 29), Fecha(2025, 2, 28), Fecha(2025, 2, 28)},
		{"29-feb en año bisiesto", Fecha(2024, 2, 29), Fecha(2028, 6, 1), Fecha(2028, 2, 29)},
		{"31-ene mantiene el 31 de enero", Fecha(2020, 1, 31), Fecha(2026, 3, 1), Fecha(2026, 1, 31)},
		{"31-ene antes del aniversario", Fecha(2020, 1, 31), Fecha(2026, 1, 30), Fecha(2025, 1, 31)},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := UltimoAniversario(c.ingreso, c.fecha); !got.Equal(c.esperado) {
				t.Fatalf("UltimoAniversario(%s, %s) = %s, esperado %s",
					c.ingreso.Format("2006-01-02"), c.fecha.Format("2006-01-02"),
					got.Format("2006-01-02"), c.esperado.Format("2006-01-02"))
			}
		})
	}
}
