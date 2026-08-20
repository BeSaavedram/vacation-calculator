package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Parametros son los valores que configuran las políticas de un tipo de
// vacación. Todo lo que en otra implementación sería una constante del código
// vive aquí: umbrales, cadencias y cantidades.
type Parametros struct {
	// Devengo
	DiasBase string `json:"dias_base,omitempty"` // "15"

	// Progresivo (solo aplica a aniversario_legal)
	ProgresivoActivo                bool   `json:"progresivo_activo,omitempty"`
	ProgresivoUmbralMeses           int    `json:"progresivo_umbral_meses,omitempty"`            // 120
	ProgresivoAntiguedadMinimaMeses int    `json:"progresivo_antiguedad_minima_meses,omitempty"` // 36
	ProgresivoCadenciaAnios         int    `json:"progresivo_cadencia_anios,omitempty"`          // 3
	ProgresivoDiasPorTramo          string `json:"progresivo_dias_por_tramo,omitempty"`          // "1"

	// Vencimiento
	NPeriodos int `json:"n_periodos,omitempty"`
	DiasFijos int `json:"dias_fijos,omitempty"`
}

// DiasBaseDecimal devuelve los días base como decimal. Cero si no está seteado
// o si el valor guardado no es parseable.
//
// NUNCA entra en pánico. Estos parámetros los escribe RRHH en una columna jsonb,
// así que "quince" o "15,5" (la coma decimal natural en Chile) son entradas
// posibles. Este accesor está en el camino caliente de toda consulta de saldo y
// del job de devengo: un pánico aquí voltearía el job a media corrida o
// devolvería 500 en cada consulta de esa empresa. Validar los valores es tarea
// de Validar(), que corre antes de persistir.
func (p Parametros) DiasBaseDecimal() decimal.Decimal {
	return decimalOCero(p.DiasBase)
}

// DiasPorTramoDecimal devuelve los días que otorga cada tramo progresivo.
// Igual que DiasBaseDecimal: nunca entra en pánico.
func (p Parametros) DiasPorTramoDecimal() decimal.Decimal {
	return decimalOCero(p.ProgresivoDiasPorTramo)
}

// decimalOCero parsea un parámetro decimal y devuelve cero si no se puede.
func decimalOCero(valor string) decimal.Decimal {
	d, err := decimal.NewFromString(valor)
	if err != nil {
		// Error ignorado a propósito: ver el comentario de DiasBaseDecimal.
		// Validar() es quien reporta el problema, en el momento de guardar.
		return decimal.Zero
	}
	return d
}

// Validar parsea todos los campos decimales y devuelve un error descriptivo que
// nombra el campo y el valor ofensivos.
//
// Es lo que llaman la pantalla de administración y la capa de repositorio ANTES
// de persistir. Los accesores no validan a propósito: para cuando se leen, el
// valor ya está guardado y fallar ahí solo voltearía la consulta.
func (p Parametros) Validar() error {
	campos := []struct{ nombre, valor string }{
		{"dias_base", p.DiasBase},
		{"progresivo_dias_por_tramo", p.ProgresivoDiasPorTramo},
	}
	for _, campo := range campos {
		if campo.valor == "" {
			continue // campo opcional no configurado
		}
		d, err := decimal.NewFromString(campo.valor)
		if err != nil {
			return fmt.Errorf("%s inválido: %q", campo.nombre, campo.valor)
		}
		if d.IsNegative() {
			return fmt.Errorf("%s no puede ser negativo: %q", campo.nombre, campo.valor)
		}
	}
	return nil
}

// TipoDeVacacion compone tres políticas intercambiables. Agregar un tipo nuevo
// es crear un registro, no escribir código.
type TipoDeVacacion struct {
	ID                  uuid.UUID
	EmpresaID           uuid.UUID
	Codigo              string
	Nombre              string
	PoliticaDevengo     CodigoDevengo
	PoliticaVencimiento CodigoVencimiento
	Parametros          Parametros
	PrioridadConsumo    int
	UnidadHabil         bool
	PagableEnFiniquito  bool
	VigenteDesde        time.Time
}
