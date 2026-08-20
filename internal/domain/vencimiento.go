package domain

import "time"

// CalcularVencimiento resuelve la fecha en que una bolsa deja de ser usable.
// La convención es EXCLUSIVA: la bolsa sirve mientras la fecha de consulta sea
// anterior a vence_el.
//
// Esta fecha se calcula UNA VEZ, al momento del otorgamiento, y se persiste.
// No se recalcula al vuelo: si la política cambia después, el saldo pasado debe
// seguir siendo explicable con las reglas que estaban vigentes.
func CalcularVencimiento(tipo TipoDeVacacion, devengadoEl time.Time) *time.Time {
	devengadoEl = SoloFecha(devengadoEl)

	switch tipo.PoliticaVencimiento {
	case VencimientoNPeriodos:
		if tipo.Parametros.NPeriodos <= 0 {
			return nil
		}
		v := devengadoEl.AddDate(tipo.Parametros.NPeriodos, 0, 0)
		return &v

	case VencimientoFinDeAnio:
		v := Fecha(devengadoEl.Year()+1, time.January, 1)
		return &v

	case VencimientoDiasFijos:
		if tipo.Parametros.DiasFijos <= 0 {
			return nil
		}
		v := devengadoEl.AddDate(0, 0, tipo.Parametros.DiasFijos)
		return &v

	case VencimientoNoVence:
		return nil

	default:
		return nil
	}
}
