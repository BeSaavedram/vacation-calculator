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

// Los feriados llegan desde la base y el calendario debe dar el mismo resultado
// venga la fecha en UTC o con zona.
func TestCalendario_NoDependeDeLaZona(t *testing.T) {
	zona := time.FixedZone("test", -4*3600)
	feriadoUTC := Fecha(2026, 9, 18)
	feriadoConZona := time.Date(2026, 9, 18, 0, 0, 0, 0, zona)

	conZona := NuevoCalendario([]time.Time{feriadoConZona})
	enUTC := NuevoCalendario([]time.Time{feriadoUTC})

	if conZona.EsHabil(feriadoUTC) {
		t.Fatal("un feriado cargado con zona debe seguir siendo inhábil")
	}
	if enUTC.EsHabil(feriadoConZona) {
		t.Fatal("consultar con zona un feriado cargado en UTC debe dar inhábil")
	}
	if got, esperado := conZona.EsHabil(feriadoConZona), enUTC.EsHabil(feriadoUTC); got != esperado {
		t.Fatalf("EsHabil con zona = %v, en UTC = %v: deben coincidir", got, esperado)
	}
}
