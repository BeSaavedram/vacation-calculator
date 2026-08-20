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

// UltimoAniversario devuelve el aniversario de ingreso más reciente que ya
// ocurrió a la fecha dada. Si la fecha ES el aniversario, devuelve esa fecha.
func UltimoAniversario(ingreso, fecha time.Time) time.Time {
	aniversario := Fecha(fecha.Year(), ingreso.Month(), ingreso.Day())
	if aniversario.After(fecha) {
		aniversario = aniversario.AddDate(-1, 0, 0)
	}
	return aniversario
}

// EsAniversario indica si la fecha cae exactamente en un aniversario de
// ingreso posterior al ingreso mismo.
func EsAniversario(ingreso, fecha time.Time) bool {
	if !fecha.After(ingreso) {
		return false
	}
	return fecha.Month() == ingreso.Month() && fecha.Day() == ingreso.Day()
}

// DiasEntre cuenta los días calendario entre dos fechas.
func DiasEntre(desde, hasta time.Time) int {
	return int(hasta.Sub(desde).Hours() / 24)
}
