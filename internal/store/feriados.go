package store

import (
	"time"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// FeriadosChile devuelve los feriados legales fijos de Chile para el rango de
// años dado, más los movibles que caen en fecha fija.
//
// Los feriados movibles reales (Viernes Santo, por ejemplo) se omiten a
// propósito: para una demo, los fijos bastan para demostrar que el conteo de
// días hábiles descuenta feriados, y evitan meter un calendario litúrgico en el
// código. En producción esta tabla se carga desde una fuente oficial.
func FeriadosChile(desdeAnio, hastaAnio int) []Feriado {
	fijos := []struct {
		mes    time.Month
		dia    int
		nombre string
	}{
		{time.January, 1, "Año Nuevo"},
		{time.May, 1, "Día del Trabajo"},
		{time.May, 21, "Glorias Navales"},
		{time.June, 29, "San Pedro y San Pablo"},
		{time.July, 16, "Virgen del Carmen"},
		{time.August, 15, "Asunción de la Virgen"},
		{time.September, 18, "Independencia Nacional"},
		{time.September, 19, "Glorias del Ejército"},
		{time.October, 12, "Encuentro de Dos Mundos"},
		{time.October, 31, "Iglesias Evangélicas"},
		{time.November, 1, "Día de Todos los Santos"},
		{time.December, 8, "Inmaculada Concepción"},
		{time.December, 25, "Navidad"},
	}

	var out []Feriado
	for anio := desdeAnio; anio <= hastaAnio; anio++ {
		for _, f := range fijos {
			out = append(out, Feriado{
				Fecha:  domain.Fecha(anio, f.mes, f.dia),
				Ambito: "CL",
				Tipo:   "feriado_legal",
				Nombre: f.nombre,
			})
		}
	}
	return out
}
