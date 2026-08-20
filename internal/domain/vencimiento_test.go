package domain

import (
	"testing"
	"time"
)

func TestCalcularVencimiento(t *testing.T) {
	casos := []struct {
		nombre      string
		politica    CodigoVencimiento
		params      Parametros
		devengadoEl time.Time
		esperado    *time.Time
	}{
		{
			nombre:   "dos períodos: el feriado legal no se acumula más de dos años",
			politica: VencimientoNPeriodos, params: Parametros{NPeriodos: 2},
			devengadoEl: Fecha(2024, 4, 15),
			esperado:    ptrFecha(Fecha(2026, 4, 15)),
		},
		{
			nombre:   "fin de año calendario: los administrativos mueren el 31 de diciembre",
			politica: VencimientoFinDeAnio, params: Parametros{},
			devengadoEl: Fecha(2026, 1, 1),
			esperado:    ptrFecha(Fecha(2027, 1, 1)),
		},
		{
			nombre:   "días fijos",
			politica: VencimientoDiasFijos, params: Parametros{DiasFijos: 90},
			devengadoEl: Fecha(2026, 1, 1),
			esperado:    ptrFecha(Fecha(2026, 4, 1)),
		},
		{
			nombre:   "no vence",
			politica: VencimientoNoVence, params: Parametros{},
			devengadoEl: Fecha(2026, 1, 1),
			esperado:    nil,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			tipo := TipoDeVacacion{PoliticaVencimiento: c.politica, Parametros: c.params}
			got := CalcularVencimiento(tipo, c.devengadoEl)

			switch {
			case c.esperado == nil && got != nil:
				t.Fatalf("esperaba nil, dio %s", got.Format("2006-01-02"))
			case c.esperado != nil && got == nil:
				t.Fatalf("esperaba %s, dio nil", c.esperado.Format("2006-01-02"))
			case c.esperado != nil && !got.Equal(*c.esperado):
				t.Fatalf("dio %s, esperado %s",
					got.Format("2006-01-02"), c.esperado.Format("2006-01-02"))
			}
		})
	}
}

func ptrFecha(f time.Time) *time.Time { return &f }

// vence_el se calcula sobre el aniversario y se persiste: debe seguir la misma
// convención de fin de mes que el aniversario del que sale.
func TestCalcularVencimiento_RecortaElFinDeMes(t *testing.T) {
	tipo := TipoDeVacacion{
		PoliticaVencimiento: VencimientoNPeriodos,
		Parametros:          Parametros{NPeriodos: 2},
	}

	got := CalcularVencimiento(tipo, Fecha(2028, 2, 29))

	if got == nil {
		t.Fatal("esperaba una fecha de vencimiento, dio nil")
	}
	if !got.Equal(Fecha(2030, 2, 28)) {
		t.Fatalf("VenceEl = %s, esperado 2030-02-28: 2030 no es bisiesto",
			got.Format("2006-01-02"))
	}
}
