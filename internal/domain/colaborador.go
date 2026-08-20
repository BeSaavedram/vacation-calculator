package domain

import (
	"time"

	"github.com/google/uuid"
)

// Colaborador reúne los datos de entrada que alimentan el cálculo. No guarda
// saldo: el saldo se proyecta desde el ledger.
type Colaborador struct {
	ID                     uuid.UUID
	EmpresaID              uuid.UUID
	Nombre                 string
	Email                  string
	Rol                    Rol
	FechaIngreso           time.Time
	FechaTermino           *time.Time
	MesesExperienciaPrevia int
	Jornada                string
}

// MesesAntiguedadAl son los meses completos con el empleador actual.
func (c Colaborador) MesesAntiguedadAl(fecha time.Time) int {
	return MesesEntre(c.FechaIngreso, fecha)
}

// MesesExperienciaTotalAl suma la antigüedad actual y la experiencia previa
// acreditada por RRHH. Es la cifra que se compara contra el umbral de las
// vacaciones progresivas.
func (c Colaborador) MesesExperienciaTotalAl(fecha time.Time) int {
	return c.MesesAntiguedadAl(fecha) + c.MesesExperienciaPrevia
}

// AniosConEmpleadorAl son los años completos con el empleador actual.
func (c Colaborador) AniosConEmpleadorAl(fecha time.Time) int {
	return c.MesesAntiguedadAl(fecha) / 12
}

// EsRRHH indica si el colaborador puede ver los datos reservados a RRHH.
func (c Colaborador) EsRRHH() bool { return c.Rol == RolRRHH }
