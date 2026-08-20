package domain

import (
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

// DiasBaseDecimal devuelve los días base como decimal. Cero si no está seteado.
func (p Parametros) DiasBaseDecimal() decimal.Decimal {
	if p.DiasBase == "" {
		return decimal.Zero
	}
	return decimal.RequireFromString(p.DiasBase)
}

// DiasPorTramoDecimal devuelve los días que otorga cada tramo progresivo.
func (p Parametros) DiasPorTramoDecimal() decimal.Decimal {
	if p.ProgresivoDiasPorTramo == "" {
		return decimal.Zero
	}
	return decimal.RequireFromString(p.ProgresivoDiasPorTramo)
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
