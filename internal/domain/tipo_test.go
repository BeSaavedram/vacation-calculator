package domain

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// Los parámetros los edita RRHH en una columna jsonb: "quince" y "15,5" son
// entradas que un humano escribe. Los accesores están en el camino caliente de
// toda consulta de saldo, así que no pueden entrar en pánico.
func TestParametros_AccesoresNoEntranEnPanico(t *testing.T) {
	casos := []struct {
		nombre string
		valor  string
	}{
		{"texto en vez de número", "quince"},
		{"decimal con coma, como se escribe en Chile", "15,5"},
		{"vacío", ""},
		{"basura", "!!"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			base := Parametros{DiasBase: c.valor}.DiasBaseDecimal()
			if !base.Equal(decimal.Zero) {
				t.Fatalf("DiasBaseDecimal(%q) = %s, esperado 0", c.valor, base)
			}
			tramo := Parametros{ProgresivoDiasPorTramo: c.valor}.DiasPorTramoDecimal()
			if !tramo.Equal(decimal.Zero) {
				t.Fatalf("DiasPorTramoDecimal(%q) = %s, esperado 0", c.valor, tramo)
			}
		})
	}
}

func TestParametros_AccesoresConValorValido(t *testing.T) {
	p := Parametros{DiasBase: "15.5", ProgresivoDiasPorTramo: "1"}
	if !p.DiasBaseDecimal().Equal(decimal.RequireFromString("15.5")) {
		t.Fatalf("DiasBaseDecimal = %s, esperado 15.5", p.DiasBaseDecimal())
	}
	if !p.DiasPorTramoDecimal().Equal(decimal.RequireFromString("1")) {
		t.Fatalf("DiasPorTramoDecimal = %s, esperado 1", p.DiasPorTramoDecimal())
	}
}

func TestParametros_Validar(t *testing.T) {
	casos := []struct {
		nombre        string
		params        Parametros
		esperaError   bool
		mencionaCampo string
		mencionaValor string
	}{
		{
			nombre: "configuración válida del feriado legal",
			params: Parametros{DiasBase: "15", ProgresivoDiasPorTramo: "1"},
		},
		{
			nombre: "campos opcionales vacíos son válidos",
			params: Parametros{},
		},
		{
			nombre:        "dias_base no parseable",
			params:        Parametros{DiasBase: "quince"},
			esperaError:   true,
			mencionaCampo: "dias_base", mencionaValor: "quince",
		},
		{
			nombre:        "dias_base con coma decimal",
			params:        Parametros{DiasBase: "15,5"},
			esperaError:   true,
			mencionaCampo: "dias_base", mencionaValor: "15,5",
		},
		{
			nombre:        "dias_base negativo",
			params:        Parametros{DiasBase: "-1"},
			esperaError:   true,
			mencionaCampo: "dias_base", mencionaValor: "-1",
		},
		{
			nombre:        "progresivo_dias_por_tramo no parseable",
			params:        Parametros{DiasBase: "15", ProgresivoDiasPorTramo: "uno"},
			esperaError:   true,
			mencionaCampo: "progresivo_dias_por_tramo", mencionaValor: "uno",
		},
		{
			nombre:        "progresivo_dias_por_tramo negativo",
			params:        Parametros{DiasBase: "15", ProgresivoDiasPorTramo: "-2"},
			esperaError:   true,
			mencionaCampo: "progresivo_dias_por_tramo", mencionaValor: "-2",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := c.params.Validar()
			if !c.esperaError {
				if err != nil {
					t.Fatalf("esperaba válido, dio error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("esperaba un error de validación, dio nil")
			}
			if !strings.Contains(err.Error(), c.mencionaCampo) {
				t.Fatalf("el error %q debe nombrar el campo %q", err, c.mencionaCampo)
			}
			if !strings.Contains(err.Error(), c.mencionaValor) {
				t.Fatalf("el error %q debe mostrar el valor %q", err, c.mencionaValor)
			}
		})
	}
}
