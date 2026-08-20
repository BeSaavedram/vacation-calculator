package domain

import (
	"testing"
	"time"
)

func TestContarHabilesDescuentaFinDeSemanaYFeriado(t *testing.T) {
	cal := NuevoCalendario([]time.Time{
		Fecha(2026, 9, 18), // fiestas patrias, viernes
		Fecha(2026, 9, 19), // sábado, ya inhábil de todos modos
	})

	got := cal.ContarHabiles(Fecha(2026, 9, 14), Fecha(2026, 9, 25))
	if got != 9 {
		t.Fatalf("ContarHabiles = %d, esperado 9", got)
	}
}

func TestEsHabil(t *testing.T) {
	cal := NuevoCalendario([]time.Time{Fecha(2026, 9, 18)})
	casos := []struct {
		nombre   string
		fecha    time.Time
		esperado bool
	}{
		{"lunes normal", Fecha(2026, 9, 14), true},
		{"sábado", Fecha(2026, 9, 12), false},
		{"domingo", Fecha(2026, 9, 13), false},
		{"feriado en día de semana", Fecha(2026, 9, 18), false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := cal.EsHabil(c.fecha); got != c.esperado {
				t.Fatalf("EsHabil(%s) = %v, esperado %v",
					c.fecha.Format("2006-01-02"), got, c.esperado)
			}
		})
	}
}

func TestContarHabilesRangoDeUnDia(t *testing.T) {
	cal := NuevoCalendario(nil)
	if got := cal.ContarHabiles(Fecha(2026, 9, 14), Fecha(2026, 9, 14)); got != 1 {
		t.Fatalf("un lunes solo debe contar 1 día hábil, dio %d", got)
	}
}
