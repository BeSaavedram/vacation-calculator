package domain

import "time"

// Calendario resuelve qué días son inhábiles. El sábado es inhábil por regla
// del negocio, no por configuración: así lo define la normativa que modela
// este sistema.
type Calendario struct {
	feriados map[string]struct{}
}

// NuevoCalendario construye un calendario a partir de las fechas de feriado.
func NuevoCalendario(feriados []time.Time) *Calendario {
	c := &Calendario{feriados: make(map[string]struct{}, len(feriados))}
	for _, f := range feriados {
		c.feriados[f.Format("2006-01-02")] = struct{}{}
	}
	return c
}

// EsHabil indica si un día cuenta como hábil: ni sábado, ni domingo, ni feriado.
func (c *Calendario) EsHabil(fecha time.Time) bool {
	switch fecha.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	_, esFeriado := c.feriados[fecha.Format("2006-01-02")]
	return !esFeriado
}

// ContarHabiles cuenta los días hábiles del rango, con ambos extremos incluidos.
func (c *Calendario) ContarHabiles(desde, hasta time.Time) int {
	total := 0
	for dia := SoloFecha(desde); !dia.After(SoloFecha(hasta)); dia = dia.AddDate(0, 0, 1) {
		if c.EsHabil(dia) {
			total++
		}
	}
	return total
}
