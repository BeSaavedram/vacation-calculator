package domain

import "time"

// Fecha construye una fecha sin hora en UTC. Todo el dominio trabaja así:
// las comparaciones de fechas nunca deben depender de la zona horaria ni de
// la hora del día en que corre un job.
func Fecha(anio int, mes time.Month, dia int) time.Time {
	return time.Date(anio, mes, dia, 0, 0, 0, 0, time.UTC)
}

// SoloFecha normaliza un time.Time descartando hora y zona.
func SoloFecha(t time.Time) time.Time {
	return Fecha(t.Year(), t.Month(), t.Day())
}

// MesesEntre cuenta los meses completos transcurridos entre dos fechas.
// Un mes está completo solo si se alcanzó el mismo día del mes.
func MesesEntre(desde, hasta time.Time) int {
	meses := (hasta.Year()-desde.Year())*12 + int(hasta.Month()) - int(desde.Month())
	if hasta.Day() < desde.Day() {
		meses--
	}
	return meses
}

// aniversarioEnAnio devuelve el día en que cae el aniversario de ingreso dentro
// del año dado.
//
// CONVENCIÓN DELIBERADA: el aniversario es el día del mes del ingreso, RECORTADO
// al último día de ese mes cuando el mes del año destino es más corto. Un
// ingreso del 29 de febrero cumple el 28 de febrero en los años comunes y el 29
// en los bisiestos; nunca se corre al 1 de marzo. La alternativa (dejar que
// time.Date desborde al mes siguiente) hacía que ese colaborador no cumpliera
// años nunca, porque ningún día del año común coincidía con el 29 de febrero.
//
// El recorte solo importa para febrero: los meses de 30 días nunca son destino
// de un ingreso del 31 porque el mes de ingreso es siempre el mismo.
//
// Es el único lugar donde se resuelve esta regla. UltimoAniversario y
// EsAniversario la usan las dos, para que no puedan discrepar.
func aniversarioEnAnio(ingreso time.Time, anio int) time.Time {
	return fechaRecortada(anio, ingreso.Month(), ingreso.Day())
}

// fechaRecortada construye una fecha recortando el día al último del mes cuando
// el mes destino es más corto. Es la ÚNICA implementación de la convención de
// fin de mes: los aniversarios y los períodos mensuales la comparten para que
// no puedan discrepar entre sí.
func fechaRecortada(anio int, mes time.Month, dia int) time.Time {
	if ultimo := diasDelMes(anio, mes); dia > ultimo {
		dia = ultimo
	}
	return Fecha(anio, mes, dia)
}

// sumarMesesRecortando suma meses respetando la convención de fin de mes: un mes
// después del 31 de enero es el 28 (o 29) de febrero, no el 3 de marzo.
//
// time.Time.AddDate normaliza el desborde hacia el mes siguiente, que es
// justamente lo que esta función evita.
func sumarMesesRecortando(fecha time.Time, meses int) time.Time {
	// El día 1 existe en todos los meses, así que mover el mes desde ahí nunca
	// desborda. El día real se aplica después, ya recortado.
	primero := Fecha(fecha.Year(), fecha.Month(), 1).AddDate(0, meses, 0)
	return fechaRecortada(primero.Year(), primero.Month(), fecha.Day())
}

// diasDelMes devuelve cuántos días tiene el mes dado del año dado.
func diasDelMes(anio int, mes time.Month) int {
	// El día 0 del mes siguiente es el último día de este mes.
	return time.Date(anio, mes+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// UltimoAniversario devuelve el aniversario de ingreso más reciente que ya
// ocurrió a la fecha dada. Si la fecha ES el aniversario, devuelve esa fecha.
func UltimoAniversario(ingreso, fecha time.Time) time.Time {
	ingreso, fecha = SoloFecha(ingreso), SoloFecha(fecha)

	aniversario := aniversarioEnAnio(ingreso, fecha.Year())
	if aniversario.After(fecha) {
		aniversario = aniversarioEnAnio(ingreso, fecha.Year()-1)
	}
	return aniversario
}

// EsAniversario indica si la fecha cae exactamente en un aniversario de
// ingreso posterior al ingreso mismo.
func EsAniversario(ingreso, fecha time.Time) bool {
	ingreso, fecha = SoloFecha(ingreso), SoloFecha(fecha)

	if !fecha.After(ingreso) {
		return false
	}
	return fecha.Equal(aniversarioEnAnio(ingreso, fecha.Year()))
}

// DiasEntre cuenta los días calendario entre dos fechas.
func DiasEntre(desde, hasta time.Time) int {
	return int(hasta.Sub(desde).Hours() / 24)
}
