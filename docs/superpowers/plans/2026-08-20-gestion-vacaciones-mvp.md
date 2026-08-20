# Gestión de Vacaciones MVP — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Construir un MVP demostrable de gestión de vacaciones donde el saldo es un dato derivado de un ledger append-only, con vista de colaborador y vista de RRHH.

**Architecture:** Dos procesos. Un API en Go con un núcleo de dominio puro (`internal/domain`, sin imports de infraestructura) que contiene todas las reglas de cálculo y se testea sin base de datos; una capa de aplicación que orquesta transacciones; repositorios pgx contra PostgreSQL. Un frontend Next.js que consume el API por HTTP. El saldo nunca se almacena: se proyecta sumando movimientos.

**Tech Stack:** Go 1.25 (`net/http` stdlib router), PostgreSQL 16 en Docker, `pgx/v5`, `shopspring/decimal`, `google/uuid`, Next.js App Router + TypeScript + Tailwind.

**Spec:** `docs/superpowers/specs/2026-08-20-gestion-vacaciones-mvp-design.md`

---

## Convenciones que aplican a todo el plan

**Fechas.** Todo el dominio trabaja con fechas sin hora, en UTC. Existe un helper `domain.Fecha(y, m, d)`. Nunca usar `time.Now()` dentro del dominio: la fecha siempre entra como parámetro. Esto es lo que hace que los jobs acepten fecha objetivo y que los tests sean deterministas.

**Decimales.** Prohibido `float64` en cualquier cálculo de días. Siempre `decimal.Decimal`. Los parámetros decimales que viajan en JSON van como string (`"15"`, no `15.0`).

**Nomenclatura.** El dominio está en español porque el dominio del problema es normativa laboral chilena y los nombres del negocio son los del documento de propuesta. Las clases de movimiento van en inglés y en mayúsculas porque así están definidas en la propuesta (`ACCRUAL`, `CONSUMPTION`, …).

**Docker.** El CLI de Docker Desktop no está en el `PATH` de zsh. El Makefile lo agrega explícitamente.

---

## Estructura de archivos

| Archivo | Responsabilidad |
|---|---|
| `docker-compose.yml` | Solo Postgres, puerto 5433 |
| `Makefile` | Comandos de la demo |
| `internal/domain/fecha.go` | Helpers de fecha sin hora y aritmética de meses |
| `internal/domain/tipos.go` | Enums: rol, clase de movimiento, estado de solicitud, códigos de política |
| `internal/domain/colaborador.go` | `Colaborador` y su antigüedad/aniversarios |
| `internal/domain/tipo.go` | `TipoDeVacacion` y sus `Parametros` |
| `internal/domain/habiles.go` | `Calendario` y conteo de días hábiles |
| `internal/domain/devengo.go` | Motor de devengo: legal, progresivo, año calendario |
| `internal/domain/vencimiento.go` | Cálculo de `vence_el` según política |
| `internal/domain/ledger.go` | `Movimiento`, `Otorgamiento`, `Bolsa` y remanentes |
| `internal/domain/saldo.go` | Proyección `[]Bolsa → Saldo` |
| `internal/domain/fifo.go` | `AsignarConsumo` |
| `internal/domain/finiquito.go` | Proporcional y finiquito |
| `internal/store/migrations/*.sql` | Esquema, GRANTs, semilla de calendario |
| `internal/store/migrate.go` | Runner de migraciones con `go:embed` |
| `internal/store/*_repo.go` | Un repositorio por agregado |
| `internal/app/saldo.go` | Caso de uso: consultar saldo con reserva de pendientes |
| `internal/app/jobs.go` | Casos de uso: devengo y vencimiento diarios |
| `internal/app/solicitudes.go` | Casos de uso: crear, aprobar (FIFO transaccional), rechazar |
| `internal/app/rrhh.go` | Casos de uso: otorgamiento manual, ABM de tipos |
| `internal/http/router.go` | Rutas y middleware de actor |
| `internal/http/handlers_*.go` | Handlers agrupados por área |
| `cmd/api/main.go` | Arranque del API |
| `cmd/migrate/main.go` | Arranque del migrador |
| `cmd/seed/main.go` | Semilla que corre el motor sobre la historia |
| `web/` | Next.js |

---

## Task 1: Esqueleto del proyecto y Postgres

**Files:**
- Create: `go.mod`, `docker-compose.yml`, `Makefile`, `.env.example`

- [ ] **Step 1: Inicializar el módulo Go y las dependencias**

```bash
go mod init github.com/BeSaavedram/vacation-calculator
go get github.com/jackc/pgx/v5@latest
go get github.com/shopspring/decimal@latest
go get github.com/google/uuid@latest
```

- [ ] **Step 2: Crear `docker-compose.yml`**

Puerto 5433 en el host para no chocar con un Postgres local si existe.

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: vacaciones-db
    environment:
      POSTGRES_USER: vacaciones
      POSTGRES_PASSWORD: vacaciones
      POSTGRES_DB: vacaciones
    ports:
      - "5433:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U vacaciones -d vacaciones"]
      interval: 2s
      timeout: 3s
      retries: 30

volumes:
  pgdata:
```

- [ ] **Step 3: Crear `.env.example`**

Dos DSN distintos a propósito. El API se conecta con un rol que **no puede** hacer `UPDATE` sobre el ledger; las migraciones y la semilla se conectan como dueño.

```bash
# Rol dueño: migraciones y semilla
DATABASE_URL_OWNER=postgres://vacaciones:vacaciones@localhost:5433/vacaciones
# Rol de aplicación: solo INSERT/SELECT sobre movimiento
DATABASE_URL=postgres://vacaciones_app:vacaciones_app@localhost:5433/vacaciones
PORT=8080
```

- [ ] **Step 4: Crear el `Makefile`**

```makefile
export PATH := $(PATH):/Applications/Docker.app/Contents/Resources/bin
include .env
export

.PHONY: db-up db-down migrate seed api web test demo-reset

db-up:
	docker compose up -d postgres
	@echo "esperando a postgres..."
	@until docker compose exec -T postgres pg_isready -U vacaciones -d vacaciones >/dev/null 2>&1; do sleep 1; done
	@echo "postgres listo en localhost:5433"

db-down:
	docker compose down -v

migrate:
	go run ./cmd/migrate

seed:
	go run ./cmd/seed

api:
	go run ./cmd/api

web:
	cd web && npm run dev

test:
	go test ./internal/... -v

demo-reset: db-down db-up migrate seed
```

- [ ] **Step 5: Verificar que Postgres levanta**

```bash
cp .env.example .env && make db-up
```

Expected: termina con `postgres listo en localhost:5433`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum docker-compose.yml Makefile .env.example
git commit -m "chore: esqueleto del proyecto y Postgres en docker"
```

---

## Task 2: Helpers de fecha

Todo el dominio depende de estos. Van primero.

**Files:**
- Create: `internal/domain/fecha.go`
- Test: `internal/domain/fecha_test.go`

- [ ] **Step 1: Escribir el test que falla**

```go
package domain

import (
	"testing"
	"time"
)

func TestMesesEntre(t *testing.T) {
	casos := []struct {
		nombre       string
		desde, hasta time.Time
		esperado     int
	}{
		{"nueve años exactos", Fecha(2017, 4, 15), Fecha(2026, 4, 15), 108},
		{"un día antes del aniversario", Fecha(2017, 4, 15), Fecha(2026, 4, 14), 107},
		{"mismo mes", Fecha(2026, 1, 1), Fecha(2026, 1, 31), 0},
		{"ocho meses y días", Fecha(2026, 1, 1), Fecha(2026, 9, 25), 8},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := MesesEntre(c.desde, c.hasta); got != c.esperado {
				t.Fatalf("MesesEntre(%s, %s) = %d, esperado %d",
					c.desde.Format("2006-01-02"), c.hasta.Format("2006-01-02"), got, c.esperado)
			}
		})
	}
}

func TestUltimoAniversario(t *testing.T) {
	casos := []struct {
		nombre          string
		ingreso, fecha  time.Time
		esperado        time.Time
	}{
		{"antes del aniversario del año", Fecha(2018, 4, 15), Fecha(2026, 2, 1), Fecha(2025, 4, 15)},
		{"después del aniversario", Fecha(2018, 4, 15), Fecha(2026, 9, 25), Fecha(2026, 4, 15)},
		{"justo en el aniversario", Fecha(2018, 4, 15), Fecha(2026, 4, 15), Fecha(2026, 4, 15)},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := UltimoAniversario(c.ingreso, c.fecha); !got.Equal(c.esperado) {
				t.Fatalf("UltimoAniversario = %s, esperado %s",
					got.Format("2006-01-02"), c.esperado.Format("2006-01-02"))
			}
		})
	}
}

func TestEsAniversario(t *testing.T) {
	ingreso := Fecha(2017, 4, 15)
	if !EsAniversario(ingreso, Fecha(2026, 4, 15)) {
		t.Fatal("2026-04-15 debería ser aniversario de un ingreso 2017-04-15")
	}
	if EsAniversario(ingreso, Fecha(2026, 4, 16)) {
		t.Fatal("2026-04-16 no debería ser aniversario")
	}
	if EsAniversario(ingreso, ingreso) {
		t.Fatal("el día de ingreso no es un aniversario")
	}
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run 'TestMesesEntre|TestUltimoAniversario|TestEsAniversario' -v`
Expected: FAIL con `undefined: Fecha`, `undefined: MesesEntre`, etc.

- [ ] **Step 3: Implementar**

```go
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
```

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -v`
Expected: PASS en los tres tests.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/fecha.go internal/domain/fecha_test.go
git commit -m "feat(domain): helpers de fecha sin hora y aritmética de meses"
```

---

## Task 3: Enums del dominio

Sin test propio: son constantes. Se validan en los tests de los tasks que las usan.

**Files:**
- Create: `internal/domain/tipos.go`

- [ ] **Step 1: Escribir el archivo completo**

```go
package domain

// Rol determina qué ve cada usuario. El colaborador ve solo su disponible;
// RRHH ve además el devengado no otorgado y el proporcional de finiquito.
type Rol string

const (
	RolColaborador Rol = "COLABORADOR"
	RolRRHH        Rol = "RRHH"
)

// ClaseMovimiento son las clases del ledger. En inglés y mayúsculas porque
// así están definidas en la propuesta.
type ClaseMovimiento string

const (
	ClaseAccrual     ClaseMovimiento = "ACCRUAL"
	ClaseConsumption ClaseMovimiento = "CONSUMPTION"
	ClaseExpiration  ClaseMovimiento = "EXPIRATION"
	ClaseAdjustment  ClaseMovimiento = "ADJUSTMENT"
	ClaseReversal    ClaseMovimiento = "REVERSAL"
	ClasePayout      ClaseMovimiento = "SETTLEMENT_PAYOUT"
	ClaseOpening     ClaseMovimiento = "OPENING_BALANCE"
)

// EstadoSolicitud es la máquina de estados de una solicitud de vacaciones.
type EstadoSolicitud string

const (
	EstadoPendiente EstadoSolicitud = "PENDIENTE"
	EstadoAprobada  EstadoSolicitud = "APROBADA"
	EstadoRechazada EstadoSolicitud = "RECHAZADA"
	EstadoCancelada EstadoSolicitud = "CANCELADA"
)

// CodigoDevengo identifica la política de devengo de un tipo de vacación.
// Los días progresivos NO son una política aparte: son un parámetro de
// aniversario_legal.
type CodigoDevengo string

const (
	DevengoAniversarioLegal CodigoDevengo = "aniversario_legal"
	DevengoAnioCalendario   CodigoDevengo = "anio_calendario"
	DevengoManual           CodigoDevengo = "manual"
)

// CodigoVencimiento identifica la política de vencimiento de un tipo.
type CodigoVencimiento string

const (
	VencimientoNoVence   CodigoVencimiento = "no_vence"
	VencimientoFinDeAnio CodigoVencimiento = "fin_de_anio"
	VencimientoNPeriodos CodigoVencimiento = "n_periodos"
	VencimientoDiasFijos CodigoVencimiento = "dias_fijos"
)

// Origen indica de dónde salió un otorgamiento.
type Origen string

const (
	OrigenAutomatico Origen = "automatico"
	OrigenManual     Origen = "manual"
	OrigenMigracion  Origen = "migracion"
)
```

- [ ] **Step 2: Verificar que compila**

Run: `go build ./internal/domain/`
Expected: sin salida (éxito).

- [ ] **Step 3: Commit**

```bash
git add internal/domain/tipos.go
git commit -m "feat(domain): enums de rol, clase de movimiento, estado y políticas"
```

---

## Task 4: Colaborador y TipoDeVacacion

**Files:**
- Create: `internal/domain/colaborador.go`, `internal/domain/tipo.go`
- Test: `internal/domain/colaborador_test.go`

- [ ] **Step 1: Escribir el test que falla**

```go
package domain

import "testing"

func TestColaboradorMesesAntiguedad(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2017, 4, 15)}
	if got := c.MesesAntiguedadAl(Fecha(2026, 4, 15)); got != 108 {
		t.Fatalf("MesesAntiguedadAl = %d, esperado 108", got)
	}
}

func TestColaboradorMesesExperienciaTotal(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2017, 4, 15), MesesExperienciaPrevia: 24}
	// 96 meses con el empleador + 24 previos = 120 justo en el umbral legal
	if got := c.MesesExperienciaTotalAl(Fecha(2025, 4, 15)); got != 120 {
		t.Fatalf("MesesExperienciaTotalAl = %d, esperado 120", got)
	}
}

func TestColaboradorAniosConEmpleador(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2017, 4, 15)}
	if got := c.AniosConEmpleadorAl(Fecha(2026, 4, 15)); got != 9 {
		t.Fatalf("AniosConEmpleadorAl = %d, esperado 9", got)
	}
	if got := c.AniosConEmpleadorAl(Fecha(2026, 4, 14)); got != 8 {
		t.Fatalf("un día antes del aniversario debe dar 8, dio %d", got)
	}
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run TestColaborador -v`
Expected: FAIL con `undefined: Colaborador`.

- [ ] **Step 3: Implementar `internal/domain/colaborador.go`**

```go
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
```

- [ ] **Step 4: Implementar `internal/domain/tipo.go`**

Los parámetros decimales viajan como string para que nunca pasen por `float64`.

```go
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
```

- [ ] **Step 5: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/colaborador.go internal/domain/tipo.go internal/domain/colaborador_test.go
git commit -m "feat(domain): Colaborador y TipoDeVacacion con políticas parametrizadas"
```

---

## Task 5: Contador de días hábiles

**Files:**
- Create: `internal/domain/habiles.go`
- Test: `internal/domain/habiles_test.go`

- [ ] **Step 1: Escribir el test que falla**

El rango 2026-09-14 (lunes) a 2026-09-25 (viernes) tiene 10 días de lunes a viernes. El 18 de septiembre es feriado legal en Chile y cae viernes; el 19 cae sábado y ya está excluido por ser fin de semana. Resultado: 9 días hábiles.

```go
package domain

import "testing"

func TestContarHabilesDescuentaFinDeSemanaYFeriado(t *testing.T) {
	cal := NuevoCalendario([]time.Time{
		Fecha(2026, 9, 18), // fiestas patrias, viernes
		Fecha(2026, 9, 19), // sábado, ya inhábil de todos modos
	})

	got := cal.ContarHabiles(Fecha(2026, 9, 14), Fecha(2026, 9, 25))
	if got != 9 {
		t.Fatalf("ContarHabiles = %d, esperado 9", got)
	}
}

func TestEsHabil(t *testing.T) {
	cal := NuevoCalendario([]time.Time{Fecha(2026, 9, 18)})
	casos := []struct {
		nombre   string
		fecha    time.Time
		esperado bool
	}{
		{"lunes normal", Fecha(2026, 9, 14), true},
		{"sábado", Fecha(2026, 9, 12), false},
		{"domingo", Fecha(2026, 9, 13), false},
		{"feriado en día de semana", Fecha(2026, 9, 18), false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := cal.EsHabil(c.fecha); got != c.esperado {
				t.Fatalf("EsHabil(%s) = %v, esperado %v",
					c.fecha.Format("2006-01-02"), got, c.esperado)
			}
		})
	}
}

func TestContarHabilesRangoDeUnDia(t *testing.T) {
	cal := NuevoCalendario(nil)
	if got := cal.ContarHabiles(Fecha(2026, 9, 14), Fecha(2026, 9, 14)); got != 1 {
		t.Fatalf("un lunes solo debe contar 1 día hábil, dio %d", got)
	}
}
```

Agregar el import de `time` al inicio del archivo de test:

```go
import (
	"testing"
	"time"
)
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run 'TestContarHabiles|TestEsHabil' -v`
Expected: FAIL con `undefined: NuevoCalendario`.

- [ ] **Step 3: Implementar**

```go
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
```

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/habiles.go internal/domain/habiles_test.go
git commit -m "feat(domain): contador de días hábiles con sábado inhábil y feriados"
```

---

## Task 6: Motor de devengo — feriado legal y progresivo

Este es el corazón de los Requisitos 1 y 2. Los dos casos de referencia del documento se testean aquí.

**Files:**
- Create: `internal/domain/devengo.go`
- Test: `internal/domain/devengo_test.go`

- [ ] **Step 1: Escribir el test que falla**

```go
package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// tipoLegal replica la configuración del feriado legal chileno.
func tipoLegal() TipoDeVacacion {
	return TipoDeVacacion{
		Codigo:              "FERIADO_LEGAL",
		PoliticaDevengo:     DevengoAniversarioLegal,
		PoliticaVencimiento: VencimientoNPeriodos,
		PrioridadConsumo:    10,
		UnidadHabil:         true,
		PagableEnFiniquito:  true,
		Parametros: Parametros{
			DiasBase:                        "15",
			ProgresivoActivo:                true,
			ProgresivoUmbralMeses:           120,
			ProgresivoAntiguedadMinimaMeses: 36,
			ProgresivoCadenciaAnios:         3,
			ProgresivoDiasPorTramo:          "1",
			NPeriodos:                       2,
		},
	}
}

// carlos es el colaborador antiguo de la demo: ingreso 2017-04-15 con 24 meses
// de experiencia previa acreditada.
func carlos() Colaborador {
	return Colaborador{
		FechaIngreso:           Fecha(2017, 4, 15),
		MesesExperienciaPrevia: 24,
	}
}

// CASO DE REFERENCIA DEL DOCUMENTO: 9 años con el empleador actual dan 3 días
// progresivos, para un total de 18 días.
func TestDevengo_NueveAniosDanDieciochoDias(t *testing.T) {
	res, hubo := Devengar(tipoLegal(), carlos(), Fecha(2026, 4, 15))

	if !hubo {
		t.Fatal("el 2026-04-15 es aniversario de ingreso: debía devengar")
	}
	if !res.Dias.Equal(decimal.RequireFromString("18")) {
		t.Fatalf("Dias = %s, esperado 18", res.Dias)
	}
	if !res.DiasProgresivo.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("DiasProgresivo = %s, esperado 3", res.DiasProgresivo)
	}
}

func TestDevengo_ProgresivoEnLosBordes(t *testing.T) {
	casos := []struct {
		nombre      string
		fecha       time.Time
		esperado    string
		explicacion string
	}{
		{"séptimo aniversario", Fecha(2024, 4, 15), "15",
			"84 + 24 = 108 meses de experiencia total, bajo el umbral de 120"},
		{"octavo aniversario, justo en el umbral", Fecha(2025, 4, 15), "17",
			"96 + 24 = 120 meses exactos; floor(8/3) = 2 días extra"},
		{"noveno aniversario", Fecha(2026, 4, 15), "18",
			"floor(9/3) = 3 días extra"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res, hubo := Devengar(tipoLegal(), carlos(), c.fecha)
			if !hubo {
				t.Fatalf("debía devengar en %s", c.fecha.Format("2006-01-02"))
			}
			if !res.Dias.Equal(decimal.RequireFromString(c.esperado)) {
				t.Fatalf("Dias = %s, esperado %s (%s)", res.Dias, c.esperado, c.explicacion)
			}
		})
	}
}

func TestDevengo_AntiguedadMinimaBloqueaElProgresivo(t *testing.T) {
	// Alguien con mucha experiencia previa pero recién llegado: supera el umbral
	// de 120 meses totales, pero no los 36 meses continuos exigidos.
	novato := Colaborador{FechaIngreso: Fecha(2024, 4, 15), MesesExperienciaPrevia: 200}

	res, hubo := Devengar(tipoLegal(), novato, Fecha(2026, 4, 15))
	if !hubo {
		t.Fatal("debía devengar")
	}
	if !res.Dias.Equal(decimal.RequireFromString("15")) {
		t.Fatalf("Dias = %s, esperado 15: no cumple los 36 meses continuos", res.Dias)
	}
}

func TestDevengo_SoloEnElAniversario(t *testing.T) {
	if _, hubo := Devengar(tipoLegal(), carlos(), Fecha(2026, 4, 16)); hubo {
		t.Fatal("no debía devengar un día que no es aniversario")
	}
	if _, hubo := Devengar(tipoLegal(), carlos(), Fecha(2017, 4, 15)); hubo {
		t.Fatal("el día de ingreso no devenga")
	}
}

func TestDevengo_PeriodoDelOtorgamiento(t *testing.T) {
	res, _ := Devengar(tipoLegal(), carlos(), Fecha(2026, 4, 15))

	if !res.PeriodoDesde.Equal(Fecha(2025, 4, 15)) {
		t.Fatalf("PeriodoDesde = %s, esperado 2025-04-15", res.PeriodoDesde.Format("2006-01-02"))
	}
	if !res.PeriodoHasta.Equal(Fecha(2026, 4, 14)) {
		t.Fatalf("PeriodoHasta = %s, esperado 2026-04-14", res.PeriodoHasta.Format("2006-01-02"))
	}
}

func TestDevengo_AnioCalendario(t *testing.T) {
	administrativo := TipoDeVacacion{
		Codigo:              "ADMINISTRATIVO",
		PoliticaDevengo:     DevengoAnioCalendario,
		PoliticaVencimiento: VencimientoFinDeAnio,
		Parametros:          Parametros{DiasBase: "6"},
	}
	c := Colaborador{FechaIngreso: Fecha(2020, 3, 1)}

	res, hubo := Devengar(administrativo, c, Fecha(2026, 1, 1))
	if !hubo {
		t.Fatal("el año calendario devenga el 1 de enero")
	}
	if !res.Dias.Equal(decimal.RequireFromString("6")) {
		t.Fatalf("Dias = %s, esperado 6", res.Dias)
	}
	if !res.PeriodoHasta.Equal(Fecha(2026, 12, 31)) {
		t.Fatalf("PeriodoHasta = %s, esperado 2026-12-31", res.PeriodoHasta.Format("2006-01-02"))
	}

	if _, hubo := Devengar(administrativo, c, Fecha(2026, 1, 2)); hubo {
		t.Fatal("el año calendario no devenga el 2 de enero")
	}
}

func TestDevengo_ManualNuncaDevengaSolo(t *testing.T) {
	rendimiento := TipoDeVacacion{
		Codigo:          "RENDIMIENTO",
		PoliticaDevengo: DevengoManual,
		Parametros:      Parametros{DiasBase: "2"},
	}
	if _, hubo := Devengar(rendimiento, carlos(), Fecha(2026, 4, 15)); hubo {
		t.Fatal("un tipo manual nunca devenga automáticamente")
	}
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run TestDevengo -v`
Expected: FAIL con `undefined: Devengar`.

- [ ] **Step 3: Implementar**

```go
package domain

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ResultadoDevengo es lo que produce el motor cuando corresponde otorgar días.
// Separa base y progresivo porque RRHH necesita poder explicar de dónde salió
// cada día del otorgamiento.
type ResultadoDevengo struct {
	Dias           decimal.Decimal
	DiasBase       decimal.Decimal
	DiasProgresivo decimal.Decimal
	PeriodoDesde   time.Time
	PeriodoHasta   time.Time
	DevengadoEl    time.Time
	Detalle        string
}

// Devengar evalúa si al colaborador le corresponde un otorgamiento de este tipo
// en la fecha dada. El segundo valor indica si hubo devengo.
//
// La fecha entra como parámetro y nunca se lee del reloj: eso es lo que permite
// reejecutar el job con fecha objetivo para recuperar días no procesados.
func Devengar(tipo TipoDeVacacion, c Colaborador, fecha time.Time) (ResultadoDevengo, bool) {
	fecha = SoloFecha(fecha)

	switch tipo.PoliticaDevengo {
	case DevengoAniversarioLegal:
		return devengarAniversario(tipo, c, fecha)
	case DevengoAnioCalendario:
		return devengarAnioCalendario(tipo, c, fecha)
	case DevengoManual:
		// Los tipos manuales solo se otorgan por acción explícita de RRHH.
		return ResultadoDevengo{}, false
	default:
		return ResultadoDevengo{}, false
	}
}

func devengarAniversario(tipo TipoDeVacacion, c Colaborador, fecha time.Time) (ResultadoDevengo, bool) {
	if !EsAniversario(c.FechaIngreso, fecha) {
		return ResultadoDevengo{}, false
	}

	base := tipo.Parametros.DiasBaseDecimal()
	progresivo := DiasProgresivos(tipo.Parametros, c, fecha)

	return ResultadoDevengo{
		Dias:           base.Add(progresivo),
		DiasBase:       base,
		DiasProgresivo: progresivo,
		PeriodoDesde:   fecha.AddDate(-1, 0, 0),
		PeriodoHasta:   fecha.AddDate(0, 0, -1),
		DevengadoEl:    fecha,
		Detalle: fmt.Sprintf("aniversario %d: %s días base + %s progresivos",
			c.AniosConEmpleadorAl(fecha), base, progresivo),
	}, true
}

func devengarAnioCalendario(tipo TipoDeVacacion, c Colaborador, fecha time.Time) (ResultadoDevengo, bool) {
	if fecha.Month() != time.January || fecha.Day() != 1 {
		return ResultadoDevengo{}, false
	}
	if fecha.Before(SoloFecha(c.FechaIngreso)) {
		return ResultadoDevengo{}, false
	}

	base := tipo.Parametros.DiasBaseDecimal()
	return ResultadoDevengo{
		Dias:           base,
		DiasBase:       base,
		DiasProgresivo: decimal.Zero,
		PeriodoDesde:   Fecha(fecha.Year(), time.January, 1),
		PeriodoHasta:   Fecha(fecha.Year(), time.December, 31),
		DevengadoEl:    fecha,
		Detalle:        fmt.Sprintf("año calendario %d: %s días", fecha.Year(), base),
	}, true
}

// DiasProgresivos aplica la regla de feriado progresivo. Umbral, antigüedad
// mínima, cadencia y días por tramo son parámetros de la política: cambiar la
// norma no exige tocar este código.
func DiasProgresivos(p Parametros, c Colaborador, fecha time.Time) decimal.Decimal {
	if !p.ProgresivoActivo {
		return decimal.Zero
	}
	if c.MesesExperienciaTotalAl(fecha) < p.ProgresivoUmbralMeses {
		return decimal.Zero
	}
	if c.MesesAntiguedadAl(fecha) < p.ProgresivoAntiguedadMinimaMeses {
		return decimal.Zero
	}
	if p.ProgresivoCadenciaAnios <= 0 {
		return decimal.Zero
	}

	tramos := c.AniosConEmpleadorAl(fecha) / p.ProgresivoCadenciaAnios
	return p.DiasPorTramoDecimal().Mul(decimal.NewFromInt(int64(tramos)))
}
```

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -run TestDevengo -v`
Expected: PASS en los siete tests, incluido `TestDevengo_NueveAniosDanDieciochoDias`.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/devengo.go internal/domain/devengo_test.go
git commit -m "feat(domain): motor de devengo legal, progresivo y año calendario"
```

---

## Task 7: Políticas de vencimiento

**Files:**
- Create: `internal/domain/vencimiento.go`
- Test: `internal/domain/vencimiento_test.go`

- [ ] **Step 1: Escribir el test que falla**

`vence_el` es la primera fecha en que la bolsa **ya no sirve**. Un otorgamiento legal del 2024-04-15 con `n_periodos = 2` vence el 2026-04-15: ese día ya no se puede usar.

```go
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
			nombre: "dos períodos: el feriado legal no se acumula más de dos años",
			politica: VencimientoNPeriodos, params: Parametros{NPeriodos: 2},
			devengadoEl: Fecha(2024, 4, 15),
			esperado:    ptrFecha(Fecha(2026, 4, 15)),
		},
		{
			nombre: "fin de año calendario: los administrativos mueren el 31 de diciembre",
			politica: VencimientoFinDeAnio, params: Parametros{},
			devengadoEl: Fecha(2026, 1, 1),
			esperado:    ptrFecha(Fecha(2027, 1, 1)),
		},
		{
			nombre: "días fijos",
			politica: VencimientoDiasFijos, params: Parametros{DiasFijos: 90},
			devengadoEl: Fecha(2026, 1, 1),
			esperado:    ptrFecha(Fecha(2026, 4, 1)),
		},
		{
			nombre: "no vence",
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
```

Nota sobre el caso `fin_de_anio`: el resultado es el **1 de enero siguiente**, porque `vence_el` es exclusivo. El último día usable sigue siendo el 31 de diciembre.

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run TestCalcularVencimiento -v`
Expected: FAIL con `undefined: CalcularVencimiento`.

- [ ] **Step 3: Implementar**

```go
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
```

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/vencimiento.go internal/domain/vencimiento_test.go
git commit -m "feat(domain): políticas de vencimiento con fecha persistida"
```

---

## Task 8: Ledger, bolsas y remanentes

**Files:**
- Create: `internal/domain/ledger.go`
- Test: `internal/domain/ledger_test.go`

- [ ] **Step 1: Escribir el test que falla**

```go
package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestBolsaRemanente(t *testing.T) {
	id := uuid.New()
	b := Bolsa{
		Otorgamiento: Otorgamiento{ID: id, DiasOtorgados: decimal.RequireFromString("18")},
		Movimientos: []Movimiento{
			{OtorgamientoID: id, Clase: ClaseAccrual, Cantidad: decimal.RequireFromString("18")},
			{OtorgamientoID: id, Clase: ClaseConsumption, Cantidad: decimal.RequireFromString("-5")},
			{OtorgamientoID: id, Clase: ClaseConsumption, Cantidad: decimal.RequireFromString("-3")},
		},
	}

	if !b.Remanente().Equal(decimal.RequireFromString("10")) {
		t.Fatalf("Remanente = %s, esperado 10", b.Remanente())
	}
}

func TestBolsaVigente(t *testing.T) {
	vence := Fecha(2026, 4, 15)
	b := Bolsa{Otorgamiento: Otorgamiento{VenceEl: &vence}}

	if !b.VigenteAl(Fecha(2026, 4, 14)) {
		t.Fatal("un día antes del vencimiento la bolsa sigue vigente")
	}
	if b.VigenteAl(Fecha(2026, 4, 15)) {
		t.Fatal("el día del vencimiento la bolsa ya no sirve: vence_el es exclusivo")
	}

	sinVencimiento := Bolsa{Otorgamiento: Otorgamiento{VenceEl: nil}}
	if !sinVencimiento.VigenteAl(Fecha(2099, 1, 1)) {
		t.Fatal("una bolsa sin vencimiento siempre está vigente")
	}
}

func TestClaveIdempotencia(t *testing.T) {
	colaborador := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tipo := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	got := ClaveIdempotencia(ClaseAccrual, colaborador, tipo, Fecha(2026, 4, 15))
	esperado := "ACCRUAL:11111111-1111-1111-1111-111111111111:22222222-2222-2222-2222-222222222222:2026-04-15"

	if got != esperado {
		t.Fatalf("ClaveIdempotencia = %q, esperado %q", got, esperado)
	}
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run 'TestBolsa|TestClaveIdempotencia' -v`
Expected: FAIL con `undefined: Bolsa`.

- [ ] **Step 3: Implementar**

```go
package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Otorgamiento es un lote concreto de días: la unidad sobre la que operan el
// vencimiento y el consumo FIFO.
type Otorgamiento struct {
	ID            uuid.UUID
	EmpresaID     uuid.UUID
	ColaboradorID uuid.UUID
	TipoID        uuid.UUID
	PeriodoDesde  time.Time
	PeriodoHasta  time.Time
	DiasOtorgados decimal.Decimal
	DevengadoEl   time.Time
	VenceEl       *time.Time
	Origen        Origen
}

// Movimiento es el registro inmutable de todo cambio de saldo. Nunca se
// actualiza ni se borra: una corrección es un REVERSAL más un movimiento nuevo.
//
// FechaEfectiva es cuándo ocurrió el hecho; FechaRegistro es cuándo lo supimos.
// Guardar ambas permite responder "¿qué sabíamos el 3 de marzo?" y no solo
// "¿qué es cierto hoy?".
type Movimiento struct {
	ID                uuid.UUID
	EmpresaID         uuid.UUID
	OtorgamientoID    uuid.UUID
	SolicitudID       *uuid.UUID
	Cantidad          decimal.Decimal
	Clase             ClaseMovimiento
	FechaEfectiva     time.Time
	FechaRegistro     time.Time
	ActorID           uuid.UUID
	Motivo            string
	ClaveIdempotencia string
	ReversaDe         *uuid.UUID
}

// Bolsa es un otorgamiento junto a los movimientos que lo afectaron. El saldo
// de la bolsa no se guarda: es la suma de sus movimientos.
type Bolsa struct {
	Otorgamiento Otorgamiento
	Movimientos  []Movimiento
}

// Remanente es lo que queda en la bolsa: la suma con signo de sus movimientos.
func (b Bolsa) Remanente() decimal.Decimal {
	total := decimal.Zero
	for _, m := range b.Movimientos {
		total = total.Add(m.Cantidad)
	}
	return total
}

// VigenteAl indica si la bolsa todavía puede consumirse en la fecha dada.
// vence_el es exclusivo: el día del vencimiento la bolsa ya no sirve.
func (b Bolsa) VigenteAl(fecha time.Time) bool {
	if b.Otorgamiento.VenceEl == nil {
		return true
	}
	return SoloFecha(fecha).Before(*b.Otorgamiento.VenceEl)
}

// ClaveIdempotencia construye la clave única que impide duplicar un movimiento
// automático. Es UNIQUE en base de datos: reejecutar un job no duplica nada
// porque el INSERT colisiona.
func ClaveIdempotencia(clase ClaseMovimiento, colaboradorID, tipoID uuid.UUID, periodo time.Time) string {
	return fmt.Sprintf("%s:%s:%s:%s", clase, colaboradorID, tipoID, periodo.Format("2006-01-02"))
}

// ClaveIdempotenciaBolsa construye la clave para un movimiento que se refiere a
// una bolsa concreta, como el vencimiento.
func ClaveIdempotenciaBolsa(clase ClaseMovimiento, otorgamientoID uuid.UUID, fecha time.Time) string {
	return fmt.Sprintf("%s:%s:%s", clase, otorgamientoID, fecha.Format("2006-01-02"))
}
```

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/ledger.go internal/domain/ledger_test.go
git commit -m "feat(domain): ledger inmutable, bolsas y claves de idempotencia"
```

---

## Task 9: Asignador de consumo FIFO

Requisito 9: descontar primero los días que vencen antes, repartiendo entre bolsas cuando sea necesario.

**Files:**
- Create: `internal/domain/fifo.go`
- Test: `internal/domain/fifo_test.go`

- [ ] **Step 1: Escribir el test que falla**

```go
package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func bolsa(vence *time.Time, remanente string, prioridad int) Bolsa {
	id := uuid.New()
	return Bolsa{
		Otorgamiento: Otorgamiento{ID: id, VenceEl: vence},
		Movimientos: []Movimiento{
			{OtorgamientoID: id, Clase: ClaseAccrual, Cantidad: decimal.RequireFromString(remanente)},
		},
	}
}

func TestAsignarConsumo_RepartEntreVariasBolsas(t *testing.T) {
	pronto := Fecha(2026, 12, 31)
	tarde := Fecha(2027, 4, 15)

	bolsas := []Bolsa{
		bolsa(&tarde, "17", 10),  // deliberadamente primero en la lista
		bolsa(&pronto, "3", 10),  // pero vence antes: debe consumirse primero
	}

	asignaciones, err := AsignarConsumo(bolsas, decimal.RequireFromString("8"), Fecha(2026, 9, 1))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if len(asignaciones) != 2 {
		t.Fatalf("esperaba 2 asignaciones, dio %d", len(asignaciones))
	}
	if asignaciones[0].OtorgamientoID != bolsas[1].Otorgamiento.ID {
		t.Fatal("la primera asignación debe salir de la bolsa que vence antes")
	}
	if !asignaciones[0].Dias.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("primera asignación = %s, esperado 3", asignaciones[0].Dias)
	}
	if !asignaciones[1].Dias.Equal(decimal.RequireFromString("5")) {
		t.Fatalf("segunda asignación = %s, esperado 5", asignaciones[1].Dias)
	}
}

func TestAsignarConsumo_DesempataPorPrioridad(t *testing.T) {
	misma := Fecha(2027, 1, 1)
	baja := bolsa(&misma, "5", 20)
	alta := bolsa(&misma, "5", 10) // menor número = se consume antes

	asignaciones, err := AsignarConsumo([]Bolsa{baja, alta}, decimal.RequireFromString("2"), Fecha(2026, 9, 1))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if asignaciones[0].OtorgamientoID != alta.Otorgamiento.ID {
		t.Fatal("a igual vencimiento debe consumirse primero la de menor prioridad numérica")
	}
}

func TestAsignarConsumo_LasBolsasSinVencimientoVanAlFinal(t *testing.T) {
	vence := Fecha(2027, 1, 1)
	conVencimiento := bolsa(&vence, "4", 10)
	sinVencimiento := bolsa(nil, "10", 10)

	asignaciones, err := AsignarConsumo(
		[]Bolsa{sinVencimiento, conVencimiento}, decimal.RequireFromString("6"), Fecha(2026, 9, 1))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if asignaciones[0].OtorgamientoID != conVencimiento.Otorgamiento.ID {
		t.Fatal("la bolsa con vencimiento debe consumirse antes que la que no vence")
	}
}

func TestAsignarConsumo_IgnoraBolsasVencidasYVacias(t *testing.T) {
	vencida := Fecha(2026, 1, 1)
	viva := Fecha(2027, 1, 1)

	asignaciones, err := AsignarConsumo([]Bolsa{
		bolsa(&vencida, "10", 10), // ya venció al 2026-09-01
		bolsa(&viva, "0", 10),     // vigente pero sin remanente
		bolsa(&viva, "4", 10),
	}, decimal.RequireFromString("4"), Fecha(2026, 9, 1))

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(asignaciones) != 1 {
		t.Fatalf("esperaba 1 asignación, dio %d", len(asignaciones))
	}
}

func TestAsignarConsumo_SaldoInsuficiente(t *testing.T) {
	viva := Fecha(2027, 1, 1)

	_, err := AsignarConsumo([]Bolsa{bolsa(&viva, "3", 10)},
		decimal.RequireFromString("5"), Fecha(2026, 9, 1))

	if !errors.Is(err, ErrSaldoInsuficiente) {
		t.Fatalf("esperaba ErrSaldoInsuficiente, dio %v", err)
	}
}
```

Nota: `AsignarConsumo` recibe bolsas **ya filtradas por tipo**. La solicitud registra su tipo y el reparto FIFO ocurre entre las bolsas de ese tipo. Quien llama es responsable del filtro.

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run TestAsignarConsumo -v`
Expected: FAIL con `undefined: AsignarConsumo`.

- [ ] **Step 3: Implementar**

Nota: `Bolsa` necesita conocer la prioridad para poder ordenar. Se agrega el campo `Prioridad` a `Bolsa` en `ledger.go`, que quien arma la bolsa rellena desde el tipo.

Agregar a `internal/domain/ledger.go`, dentro de la definición de `Bolsa`:

```go
type Bolsa struct {
	Otorgamiento Otorgamiento
	Movimientos  []Movimiento
	// Prioridad se copia desde el TipoDeVacacion al armar la bolsa. Vive aquí
	// para que el ordenamiento FIFO no necesite cargar el tipo completo.
	Prioridad int
}
```

Y actualizar el helper `bolsa()` del test de fifo para que setee `Prioridad: prioridad`:

```go
func bolsa(vence *time.Time, remanente string, prioridad int) Bolsa {
	id := uuid.New()
	return Bolsa{
		Otorgamiento: Otorgamiento{ID: id, VenceEl: vence},
		Prioridad:    prioridad,
		Movimientos: []Movimiento{
			{OtorgamientoID: id, Clase: ClaseAccrual, Cantidad: decimal.RequireFromString(remanente)},
		},
	}
}
```

Crear `internal/domain/fifo.go`:

```go
package domain

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ErrSaldoInsuficiente indica que las bolsas vigentes no alcanzan a cubrir los
// días pedidos.
var ErrSaldoInsuficiente = errors.New("saldo insuficiente")

// Asignacion es un tramo de consumo contra una bolsa concreta. Cada asignación
// se convertirá en un movimiento CONSUMPTION propio, para que el historial
// muestre de qué bolsa salió cada día.
type Asignacion struct {
	OtorgamientoID uuid.UUID
	Dias           decimal.Decimal
}

// AsignarConsumo reparte los días pedidos entre las bolsas, consumiendo primero
// las que vencen antes y desempatando por prioridad del tipo.
//
// Recibe las bolsas ya filtradas por tipo de vacación: la solicitud registra su
// tipo y el reparto ocurre dentro de ese tipo.
func AsignarConsumo(bolsas []Bolsa, dias decimal.Decimal, alDia time.Time) ([]Asignacion, error) {
	if dias.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("los días a consumir deben ser positivos")
	}

	candidatas := bolsasConsumibles(bolsas, alDia)
	ordenarFIFO(candidatas)

	restante := dias
	asignaciones := make([]Asignacion, 0, len(candidatas))

	for _, b := range candidatas {
		if restante.LessThanOrEqual(decimal.Zero) {
			break
		}
		tramo := decimal.Min(b.Remanente(), restante)
		asignaciones = append(asignaciones, Asignacion{
			OtorgamientoID: b.Otorgamiento.ID,
			Dias:           tramo,
		})
		restante = restante.Sub(tramo)
	}

	if restante.GreaterThan(decimal.Zero) {
		return nil, ErrSaldoInsuficiente
	}
	return asignaciones, nil
}

// bolsasConsumibles descarta las vencidas y las que ya no tienen remanente.
func bolsasConsumibles(bolsas []Bolsa, alDia time.Time) []Bolsa {
	out := make([]Bolsa, 0, len(bolsas))
	for _, b := range bolsas {
		if b.VigenteAl(alDia) && b.Remanente().GreaterThan(decimal.Zero) {
			out = append(out, b)
		}
	}
	return out
}

// ordenarFIFO ordena por vence_el ascendente y, a igual fecha, por prioridad.
// Las bolsas sin vencimiento van al final: se consumen cuando ya no queda nada
// que esté por perderse.
func ordenarFIFO(bolsas []Bolsa) {
	sort.SliceStable(bolsas, func(i, j int) bool {
		vi, vj := bolsas[i].Otorgamiento.VenceEl, bolsas[j].Otorgamiento.VenceEl

		switch {
		case vi == nil && vj == nil:
			return bolsas[i].Prioridad < bolsas[j].Prioridad
		case vi == nil:
			return false
		case vj == nil:
			return true
		case !vi.Equal(*vj):
			return vi.Before(*vj)
		default:
			return bolsas[i].Prioridad < bolsas[j].Prioridad
		}
	})
}
```

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -v`
Expected: PASS en los cinco tests de FIFO y en todo lo anterior.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/fifo.go internal/domain/fifo_test.go internal/domain/ledger.go
git commit -m "feat(domain): asignador de consumo FIFO por vencimiento y prioridad"
```

---

## Task 10: Proporcional y finiquito

Requisito 3. El segundo caso de referencia del documento se testea aquí.

**Files:**
- Create: `internal/domain/finiquito.go`
- Test: `internal/domain/finiquito_test.go`

- [ ] **Step 1: Escribir el test que falla**

```go
package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CASO DE REFERENCIA DEL DOCUMENTO: 8 meses completos y 24 días del período en
// curso dan 8 × 1,25 + (24/30) × 1,25 = 11,00 días.
func TestProporcional_CasoDeReferencia(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 1, 1)}

	p := CalcularProporcional(c, Fecha(2026, 9, 25))

	if p.MesesCompletos != 8 {
		t.Fatalf("MesesCompletos = %d, esperado 8", p.MesesCompletos)
	}
	if p.DiasRestantes != 24 {
		t.Fatalf("DiasRestantes = %d, esperado 24", p.DiasRestantes)
	}
	if !p.Dias.Equal(decimal.RequireFromString("11")) {
		t.Fatalf("Dias = %s, esperado 11.00", p.Dias)
	}
}

func TestProporcional_JustoEnElAniversarioEsCero(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 4, 15)}

	p := CalcularProporcional(c, Fecha(2026, 4, 15))

	if !p.Dias.Equal(decimal.Zero) {
		t.Fatalf("Dias = %s, esperado 0: el período en curso recién empieza", p.Dias)
	}
}

func TestProporcional_UnMesExacto(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 1, 1)}

	p := CalcularProporcional(c, Fecha(2026, 2, 1))

	if !p.Dias.Equal(decimal.RequireFromString("1.25")) {
		t.Fatalf("Dias = %s, esperado 1.25", p.Dias)
	}
}

func TestCalcularFiniquito_SumaProporcionalYDisponiblePagable(t *testing.T) {
	c := Colaborador{FechaIngreso: Fecha(2018, 1, 1)}
	tipoPagable := uuid.New()
	tipoNoPagable := uuid.New()
	vence := Fecha(2028, 1, 1)

	bolsas := []Bolsa{
		bolsaDeTipo(tipoPagable, &vence, "10"),
		bolsaDeTipo(tipoNoPagable, &vence, "6"), // administrativo: no se paga
	}
	pagables := map[uuid.UUID]bool{tipoPagable: true, tipoNoPagable: false}

	f := CalcularFiniquito(c, bolsas, pagables, Fecha(2026, 9, 25))

	if !f.Proporcional.Dias.Equal(decimal.RequireFromString("11")) {
		t.Fatalf("Proporcional = %s, esperado 11.00", f.Proporcional.Dias)
	}
	if !f.DisponiblePagable.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("DisponiblePagable = %s, esperado 10: el no pagable no cuenta", f.DisponiblePagable)
	}
	if !f.Total.Equal(decimal.RequireFromString("21")) {
		t.Fatalf("Total = %s, esperado 21.00", f.Total)
	}
}

func bolsaDeTipo(tipoID uuid.UUID, vence *time.Time, remanente string) Bolsa {
	id := uuid.New()
	return Bolsa{
		Otorgamiento: Otorgamiento{ID: id, TipoID: tipoID, VenceEl: vence},
		Movimientos: []Movimiento{
			{OtorgamientoID: id, Clase: ClaseAccrual, Cantidad: decimal.RequireFromString(remanente)},
		},
	}
}
```

Agregar `"time"` a los imports del archivo de test.

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run 'TestProporcional|TestCalcularFiniquito' -v`
Expected: FAIL con `undefined: CalcularProporcional`.

- [ ] **Step 3: Implementar**

```go
package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// DiasPorMes es la tasa de devengo continuo: 15 días hábiles al año divididos
// en 12 meses.
var DiasPorMes = decimal.RequireFromString("1.25")

var treinta = decimal.NewFromInt(30)

// Proporcional es el derecho devengado del período en curso que todavía no se
// otorgó. Responde la pregunta de RRHH: "¿cuánto le debo si lo desvinculo hoy?".
//
// Es el mismo cálculo que alimenta el campo devengado_no_otorgado del saldo.
type Proporcional struct {
	PeriodoDesde   time.Time
	Hasta          time.Time
	MesesCompletos int
	DiasRestantes  int
	Dias           decimal.Decimal
}

// CalcularProporcional aplica meses × 1,25 + (días / 30) × 1,25 sobre el período
// en curso, prorrateando la fracción sobre base 30.
func CalcularProporcional(c Colaborador, fecha time.Time) Proporcional {
	fecha = SoloFecha(fecha)
	inicio := UltimoAniversario(c.FechaIngreso, fecha)

	meses := MesesEntre(inicio, fecha)
	baseDelMes := inicio.AddDate(0, meses, 0)
	dias := DiasEntre(baseDelMes, fecha)

	porMeses := DiasPorMes.Mul(decimal.NewFromInt(int64(meses)))
	porDias := decimal.NewFromInt(int64(dias)).Div(treinta).Mul(DiasPorMes)

	return Proporcional{
		PeriodoDesde:   inicio,
		Hasta:          fecha,
		MesesCompletos: meses,
		DiasRestantes:  dias,
		Dias:           porMeses.Add(porDias).Round(2),
	}
}

// Finiquito es el desglose completo de lo que se le debe a un colaborador si se
// desvincula en la fecha dada. Es una consulta de solo lectura: no escribe
// movimientos. El SETTLEMENT_PAYOUT se escribe recién al confirmarse la
// desvinculación, que está fuera del alcance de este MVP.
type Finiquito struct {
	Proporcional      Proporcional
	DisponiblePagable decimal.Decimal
	Total             decimal.Decimal
}

// CalcularFiniquito suma el proporcional del período en curso y el disponible de
// los tipos marcados como pagables.
func CalcularFiniquito(
	c Colaborador,
	bolsas []Bolsa,
	pagablePorTipo map[uuid.UUID]bool,
	fecha time.Time,
) Finiquito {
	proporcional := CalcularProporcional(c, fecha)

	disponible := decimal.Zero
	for _, b := range bolsas {
		if !pagablePorTipo[b.Otorgamiento.TipoID] {
			continue
		}
		if !b.VigenteAl(fecha) {
			continue
		}
		disponible = disponible.Add(b.Remanente())
	}

	return Finiquito{
		Proporcional:      proporcional,
		DisponiblePagable: disponible.Round(2),
		Total:             proporcional.Dias.Add(disponible).Round(2),
	}
}
```

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -v`
Expected: PASS. `TestProporcional_CasoDeReferencia` confirma el `11.00` del documento.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/finiquito.go internal/domain/finiquito_test.go
git commit -m "feat(domain): feriado proporcional y cálculo de finiquito"
```

---

## Task 11: Proyección del saldo

**Files:**
- Create: `internal/domain/saldo.go`
- Test: `internal/domain/saldo_test.go`

- [ ] **Step 1: Escribir el test que falla**

```go
package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestProyectarSaldo_SumaSoloBolsasVigentes(t *testing.T) {
	tipoID := uuid.New()
	vencida := Fecha(2026, 1, 1)
	viva := Fecha(2027, 4, 15)

	bolsas := []Bolsa{
		bolsaDeTipo(tipoID, &vencida, "15"), // ya venció
		bolsaDeTipo(tipoID, &viva, "18"),
	}
	tipos := map[uuid.UUID]TipoDeVacacion{
		tipoID: {ID: tipoID, Codigo: "FERIADO_LEGAL", Nombre: "Feriado legal"},
	}

	saldo := ProyectarSaldo(bolsas, tipos, Fecha(2026, 9, 1))

	if len(saldo.PorTipo) != 1 {
		t.Fatalf("esperaba 1 tipo, dio %d", len(saldo.PorTipo))
	}
	if !saldo.PorTipo[0].Disponible.Equal(decimal.RequireFromString("18")) {
		t.Fatalf("Disponible = %s, esperado 18", saldo.PorTipo[0].Disponible)
	}
}

func TestProyectarSaldo_MarcaLoQueEstaPorVencer(t *testing.T) {
	tipoID := uuid.New()
	pronto := Fecha(2026, 10, 15) // dentro de 44 días
	lejos := Fecha(2028, 1, 1)

	bolsas := []Bolsa{
		bolsaDeTipo(tipoID, &pronto, "3"),
		bolsaDeTipo(tipoID, &lejos, "18"),
	}
	tipos := map[uuid.UUID]TipoDeVacacion{tipoID: {ID: tipoID, Codigo: "FERIADO_LEGAL"}}

	saldo := ProyectarSaldo(bolsas, tipos, Fecha(2026, 9, 1))

	if len(saldo.PorTipo[0].PorVencer) != 1 {
		t.Fatalf("esperaba 1 bolsa por vencer, dio %d", len(saldo.PorTipo[0].PorVencer))
	}
	if !saldo.PorTipo[0].PorVencer[0].Dias.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("PorVencer = %s, esperado 3", saldo.PorTipo[0].PorVencer[0].Dias)
	}
}

func TestProyectarSaldo_AgrupaPorTipo(t *testing.T) {
	legal, administrativo := uuid.New(), uuid.New()
	viva := Fecha(2028, 1, 1)

	bolsas := []Bolsa{
		bolsaDeTipo(legal, &viva, "18"),
		bolsaDeTipo(administrativo, &viva, "6"),
	}
	tipos := map[uuid.UUID]TipoDeVacacion{
		legal:          {ID: legal, Codigo: "FERIADO_LEGAL", PrioridadConsumo: 10},
		administrativo: {ID: administrativo, Codigo: "ADMINISTRATIVO", PrioridadConsumo: 20},
	}

	saldo := ProyectarSaldo(bolsas, tipos, Fecha(2026, 9, 1))

	if len(saldo.PorTipo) != 2 {
		t.Fatalf("esperaba 2 tipos, dio %d", len(saldo.PorTipo))
	}
	// Orden estable por prioridad de consumo
	if saldo.PorTipo[0].TipoCodigo != "FERIADO_LEGAL" {
		t.Fatalf("primer tipo = %s, esperado FERIADO_LEGAL", saldo.PorTipo[0].TipoCodigo)
	}
	if !saldo.Total().Equal(decimal.RequireFromString("24")) {
		t.Fatalf("Total = %s, esperado 24", saldo.Total())
	}
}
```

- [ ] **Step 2: Correr el test para verificar que falla**

Run: `go test ./internal/domain/ -run TestProyectarSaldo -v`
Expected: FAIL con `undefined: ProyectarSaldo`.

- [ ] **Step 3: Implementar**

```go
package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// DiasParaAvisoDeVencimiento define la ventana en la que una bolsa se considera
// "próxima a vencer" y se destaca en la interfaz.
const DiasParaAvisoDeVencimiento = 90

// BolsaPorVencer es un lote de días que está por perderse.
type BolsaPorVencer struct {
	OtorgamientoID uuid.UUID
	Dias           decimal.Decimal
	VenceEl        time.Time
}

// SaldoPorTipo es el disponible de un tipo de vacación concreto.
type SaldoPorTipo struct {
	TipoID     uuid.UUID
	TipoCodigo string
	TipoNombre string
	Disponible decimal.Decimal
	PorVencer  []BolsaPorVencer
}

// Saldo es una PROYECCIÓN, no un dato almacenado. Se recalcula sumando el
// ledger en cada consulta. No existe en ninguna tabla como número editable:
// esa es la decisión central de este sistema.
type Saldo struct {
	ColaboradorID uuid.UUID
	AlDia         time.Time
	PorTipo       []SaldoPorTipo
}

// Total suma el disponible de todos los tipos.
func (s Saldo) Total() decimal.Decimal {
	total := decimal.Zero
	for _, t := range s.PorTipo {
		total = total.Add(t.Disponible)
	}
	return total
}

// ProyectarSaldo reduce las bolsas de un colaborador a su saldo disponible por
// tipo, descartando lo vencido y destacando lo que está por vencer.
func ProyectarSaldo(bolsas []Bolsa, tipos map[uuid.UUID]TipoDeVacacion, alDia time.Time) Saldo {
	alDia = SoloFecha(alDia)
	limiteAviso := alDia.AddDate(0, 0, DiasParaAvisoDeVencimiento)

	porTipo := make(map[uuid.UUID]*SaldoPorTipo)
	var colaboradorID uuid.UUID

	for _, b := range bolsas {
		colaboradorID = b.Otorgamiento.ColaboradorID

		if !b.VigenteAl(alDia) {
			continue
		}
		remanente := b.Remanente()
		if remanente.LessThanOrEqual(decimal.Zero) {
			continue
		}

		tipoID := b.Otorgamiento.TipoID
		entrada, existe := porTipo[tipoID]
		if !existe {
			tipo := tipos[tipoID]
			entrada = &SaldoPorTipo{
				TipoID:     tipoID,
				TipoCodigo: tipo.Codigo,
				TipoNombre: tipo.Nombre,
				Disponible: decimal.Zero,
			}
			porTipo[tipoID] = entrada
		}

		entrada.Disponible = entrada.Disponible.Add(remanente)

		if v := b.Otorgamiento.VenceEl; v != nil && v.Before(limiteAviso) {
			entrada.PorVencer = append(entrada.PorVencer, BolsaPorVencer{
				OtorgamientoID: b.Otorgamiento.ID,
				Dias:           remanente,
				VenceEl:        *v,
			})
		}
	}

	saldo := Saldo{ColaboradorID: colaboradorID, AlDia: alDia}
	for _, entrada := range porTipo {
		sort.Slice(entrada.PorVencer, func(i, j int) bool {
			return entrada.PorVencer[i].VenceEl.Before(entrada.PorVencer[j].VenceEl)
		})
		saldo.PorTipo = append(saldo.PorTipo, *entrada)
	}

	// Orden estable por prioridad de consumo: primero lo que se gasta primero.
	sort.Slice(saldo.PorTipo, func(i, j int) bool {
		pi := tipos[saldo.PorTipo[i].TipoID].PrioridadConsumo
		pj := tipos[saldo.PorTipo[j].TipoID].PrioridadConsumo
		if pi != pj {
			return pi < pj
		}
		return saldo.PorTipo[i].TipoCodigo < saldo.PorTipo[j].TipoCodigo
	})

	return saldo
}
```

Nota: `saldo_test.go` usa el helper `bolsaDeTipo` definido en `finiquito_test.go`. Están en el mismo paquete, así que se comparte sin importar nada.

- [ ] **Step 4: Correr el test para verificar que pasa**

Run: `go test ./internal/domain/ -v`
Expected: PASS en toda la suite de dominio.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/saldo.go internal/domain/saldo_test.go
git commit -m "feat(domain): proyección del saldo desde el ledger"
```

---

## Task 12: Esquema de base de datos y migrador

**Files:**
- Create: `internal/store/migrations/001_esquema.sql`, `internal/store/migrations/002_permisos.sql`, `internal/store/migrate.go`, `cmd/migrate/main.go`

- [ ] **Step 1: Crear `internal/store/migrations/001_esquema.sql`**

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE empresa (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    razon_social  text NOT NULL
);

CREATE TABLE colaborador (
    id                        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id                uuid NOT NULL REFERENCES empresa(id),
    nombre                    text NOT NULL,
    email                     text NOT NULL,
    rol                       text NOT NULL CHECK (rol IN ('COLABORADOR', 'RRHH')),
    fecha_ingreso             date NOT NULL,
    fecha_termino             date,
    meses_experiencia_previa  int  NOT NULL DEFAULT 0,
    jornada                   text NOT NULL DEFAULT 'completa'
);

CREATE TABLE tipo_de_vacacion (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id            uuid NOT NULL REFERENCES empresa(id),
    codigo                text NOT NULL,
    nombre                text NOT NULL,
    politica_devengo      text NOT NULL
        CHECK (politica_devengo IN ('aniversario_legal', 'anio_calendario', 'manual')),
    politica_vencimiento  text NOT NULL
        CHECK (politica_vencimiento IN ('no_vence', 'fin_de_anio', 'n_periodos', 'dias_fijos')),
    parametros            jsonb NOT NULL DEFAULT '{}'::jsonb,
    prioridad_consumo     int  NOT NULL DEFAULT 100,
    unidad_habil          boolean NOT NULL DEFAULT true,
    pagable_en_finiquito  boolean NOT NULL DEFAULT false,
    vigente_desde         date NOT NULL DEFAULT '2000-01-01',
    UNIQUE (empresa_id, codigo)
);

CREATE TABLE otorgamiento (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      uuid NOT NULL REFERENCES empresa(id),
    colaborador_id  uuid NOT NULL REFERENCES colaborador(id),
    tipo_id         uuid NOT NULL REFERENCES tipo_de_vacacion(id),
    periodo_desde   date NOT NULL,
    periodo_hasta   date NOT NULL,
    dias_otorgados  DECIMAL(6,2) NOT NULL,
    devengado_el    date NOT NULL,
    vence_el        date,
    origen          text NOT NULL CHECK (origen IN ('automatico', 'manual', 'migracion'))
);

CREATE INDEX idx_otorgamiento_colaborador_tipo_vence
    ON otorgamiento (colaborador_id, tipo_id, vence_el);

CREATE TABLE solicitud_de_vacaciones (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      uuid NOT NULL REFERENCES empresa(id),
    colaborador_id  uuid NOT NULL REFERENCES colaborador(id),
    tipo_id         uuid NOT NULL REFERENCES tipo_de_vacacion(id),
    desde           date NOT NULL,
    hasta           date NOT NULL,
    dias_habiles    DECIMAL(6,2) NOT NULL,
    estado          text NOT NULL
        CHECK (estado IN ('PENDIENTE', 'APROBADA', 'RECHAZADA', 'CANCELADA')),
    aprobador_id    uuid REFERENCES colaborador(id),
    decidido_el     timestamptz,
    creada_el       timestamptz NOT NULL DEFAULT now(),
    CHECK (hasta >= desde)
);

CREATE INDEX idx_solicitud_colaborador_estado
    ON solicitud_de_vacaciones (colaborador_id, estado);

-- El ledger. Append-only por diseño y por permisos: ver 002_permisos.sql.
CREATE TABLE movimiento (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id           uuid NOT NULL REFERENCES empresa(id),
    otorgamiento_id      uuid NOT NULL REFERENCES otorgamiento(id),
    solicitud_id         uuid REFERENCES solicitud_de_vacaciones(id),
    cantidad             DECIMAL(6,2) NOT NULL,
    clase                text NOT NULL CHECK (clase IN (
        'ACCRUAL', 'CONSUMPTION', 'EXPIRATION', 'ADJUSTMENT',
        'REVERSAL', 'SETTLEMENT_PAYOUT', 'OPENING_BALANCE')),
    fecha_efectiva       date NOT NULL,
    fecha_registro       timestamptz NOT NULL DEFAULT now(),
    actor_id             uuid NOT NULL REFERENCES colaborador(id),
    motivo               text NOT NULL,
    clave_idempotencia   text NOT NULL UNIQUE,
    reversa_de           uuid REFERENCES movimiento(id)
);

CREATE INDEX idx_movimiento_otorgamiento ON movimiento (otorgamiento_id);
CREATE INDEX idx_movimiento_fecha_efectiva ON movimiento (fecha_efectiva);

CREATE TABLE calendario_laboral (
    id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    fecha   date NOT NULL,
    ambito  text NOT NULL DEFAULT 'CL',
    tipo    text NOT NULL CHECK (tipo IN ('feriado_legal', 'feriado_regional')),
    nombre  text NOT NULL DEFAULT '',
    UNIQUE (fecha, ambito)
);

CREATE INDEX idx_calendario_fecha ON calendario_laboral (fecha);
```

- [ ] **Step 2: Crear `internal/store/migrations/002_permisos.sql`**

Esto es lo que hace que la inmutabilidad sea real y no una promesa del código de aplicación.

```sql
-- El rol de aplicación NO PUEDE actualizar ni borrar movimientos. Un UPDATE
-- sobre el ledger falla en el motor de base de datos, aunque alguien lo escriba
-- por error en el código Go. Es demostrable en vivo desde psql.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'vacaciones_app') THEN
        CREATE ROLE vacaciones_app LOGIN PASSWORD 'vacaciones_app';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE vacaciones TO vacaciones_app;
GRANT USAGE ON SCHEMA public TO vacaciones_app;

GRANT SELECT, INSERT, UPDATE, DELETE ON
    empresa, colaborador, tipo_de_vacacion, otorgamiento,
    solicitud_de_vacaciones, calendario_laboral
TO vacaciones_app;

-- El ledger es la excepción deliberada: solo se puede leer e insertar.
GRANT SELECT, INSERT ON movimiento TO vacaciones_app;
REVOKE UPDATE, DELETE ON movimiento FROM vacaciones_app;
```

- [ ] **Step 3: Crear el migrador `internal/store/migrate.go`**

```go
package store

import (
	"context"
	"embed"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migraciones embed.FS

// Migrar aplica los archivos SQL de migrations/ en orden alfabético, saltando
// los que ya se aplicaron. Correrlo dos veces es seguro.
//
// Debe correr con el rol dueño de la base, no con el rol de aplicación: el rol
// de aplicación deliberadamente no tiene permisos de DDL.
func Migrar(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS migracion_aplicada (
			nombre     text PRIMARY KEY,
			aplicada_el timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("creando el registro de migraciones: %w", err)
	}

	entradas, err := migraciones.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("leyendo migraciones: %w", err)
	}

	nombres := make([]string, 0, len(entradas))
	for _, e := range entradas {
		nombres = append(nombres, e.Name())
	}
	sort.Strings(nombres)

	for _, nombre := range nombres {
		var yaEsta bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM migracion_aplicada WHERE nombre = $1)`,
			nombre).Scan(&yaEsta); err != nil {
			return err
		}
		if yaEsta {
			fmt.Printf("omitida  %s (ya aplicada)\n", nombre)
			continue
		}

		sql, err := migraciones.ReadFile("migrations/" + nombre)
		if err != nil {
			return fmt.Errorf("leyendo %s: %w", nombre, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("aplicando %s: %w", nombre, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO migracion_aplicada (nombre) VALUES ($1)`, nombre); err != nil {
			return err
		}
		fmt.Printf("aplicada %s\n", nombre)
	}
	return nil
}
```

- [ ] **Step 4: Crear `cmd/migrate/main.go`**

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

func main() {
	dsn := os.Getenv("DATABASE_URL_OWNER")
	if dsn == "" {
		log.Fatal("falta DATABASE_URL_OWNER")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("conectando: %v", err)
	}
	defer pool.Close()

	if err := store.Migrar(ctx, pool); err != nil {
		log.Fatalf("migrando: %v", err)
	}
	log.Println("migraciones aplicadas")
}
```

- [ ] **Step 5: Correr las migraciones y verificar la inmutabilidad**

```bash
make migrate
```

Expected: `aplicada 001_esquema.sql`, `aplicada 002_permisos.sql`, `migraciones aplicadas`.

Verificar que el rol de aplicación realmente no puede actualizar el ledger:

```bash
docker compose exec -T postgres psql -U vacaciones_app -d vacaciones -c "UPDATE movimiento SET cantidad = 0;"
```

Expected: `ERROR:  permission denied for table movimiento`

Si ese comando **no** falla, la migración de permisos no se aplicó y hay que corregirla antes de seguir. Este es el control que sostiene toda la propuesta.

- [ ] **Step 6: Commit**

```bash
git add internal/store cmd/migrate
git commit -m "feat(store): esquema, ledger append-only por permisos y migrador"
```

---

## Task 13: Repositorios

Un principio para toda esta capa: los decimales **nunca** pasan por `float64`. Se leen con `columna::text` y se escriben con `$n::numeric` pasando `decimal.String()`. Eso evita una dependencia extra de pgx y hace explícito el requisito no funcional.

**Files:**
- Create: `internal/store/store.go`, `internal/store/colaborador_repo.go`, `internal/store/tipo_repo.go`, `internal/store/ledger_repo.go`, `internal/store/solicitud_repo.go`, `internal/store/calendario_repo.go`

- [ ] **Step 1: Crear `internal/store/store.go`**

```go
package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier es lo que satisfacen tanto *pgxpool.Pool como pgx.Tx. Los
// repositorios lo reciben para poder correr dentro o fuera de una transacción
// sin duplicar código.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
```

- [ ] **Step 2: Crear `internal/store/colaborador_repo.go`**

```go
package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

const camposColaborador = `
	id, empresa_id, nombre, email, rol, fecha_ingreso, fecha_termino,
	meses_experiencia_previa, jornada`

func escanearColaborador(row pgx.Row) (domain.Colaborador, error) {
	var c domain.Colaborador
	err := row.Scan(&c.ID, &c.EmpresaID, &c.Nombre, &c.Email, &c.Rol,
		&c.FechaIngreso, &c.FechaTermino, &c.MesesExperienciaPrevia, &c.Jornada)
	if err != nil {
		return domain.Colaborador{}, err
	}
	c.FechaIngreso = domain.SoloFecha(c.FechaIngreso)
	return c, nil
}

// ColaboradorPorID busca un colaborador dentro de su empresa. El filtro por
// empresa_id es obligatorio en todos los repositorios, sin excepción.
func ColaboradorPorID(ctx context.Context, q Querier, empresaID, id uuid.UUID) (domain.Colaborador, error) {
	row := q.QueryRow(ctx,
		`SELECT `+camposColaborador+` FROM colaborador WHERE empresa_id = $1 AND id = $2`,
		empresaID, id)

	c, err := escanearColaborador(row)
	if err != nil {
		return domain.Colaborador{}, fmt.Errorf("colaborador %s: %w", id, err)
	}
	return c, nil
}

// ColaboradorPorIDSinEmpresa se usa solo en el middleware de actor, donde
// todavía no se sabe a qué empresa pertenece quien llama.
func ColaboradorPorIDSinEmpresa(ctx context.Context, q Querier, id uuid.UUID) (domain.Colaborador, error) {
	row := q.QueryRow(ctx, `SELECT `+camposColaborador+` FROM colaborador WHERE id = $1`, id)
	return escanearColaborador(row)
}

// ListarColaboradores devuelve todos los colaboradores de la empresa.
func ListarColaboradores(ctx context.Context, q Querier, empresaID uuid.UUID) ([]domain.Colaborador, error) {
	rows, err := q.Query(ctx,
		`SELECT `+camposColaborador+` FROM colaborador WHERE empresa_id = $1 ORDER BY nombre`,
		empresaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Colaborador
	for rows.Next() {
		c, err := escanearColaborador(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CrearColaborador inserta un colaborador. Solo lo usa la semilla.
func CrearColaborador(ctx context.Context, q Querier, c domain.Colaborador) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO colaborador (empresa_id, nombre, email, rol, fecha_ingreso,
			fecha_termino, meses_experiencia_previa, jornada)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		c.EmpresaID, c.Nombre, c.Email, c.Rol, c.FechaIngreso,
		c.FechaTermino, c.MesesExperienciaPrevia, c.Jornada,
	).Scan(&id)
	return id, err
}

// BloquearColaborador toma un lock de fila sobre el colaborador. Se llama al
// inicio de la transacción de aprobación para impedir que dos solicitudes
// concurrentes gasten los mismos días.
func BloquearColaborador(ctx context.Context, q Querier, id uuid.UUID) error {
	_, err := q.Exec(ctx, `SELECT id FROM colaborador WHERE id = $1 FOR UPDATE`, id)
	return err
}
```

Agregar `"github.com/jackc/pgx/v5"` a los imports de este archivo (lo usa `pgx.Row`).

- [ ] **Step 3: Crear `internal/store/tipo_repo.go`**

```go
package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

const camposTipo = `
	id, empresa_id, codigo, nombre, politica_devengo, politica_vencimiento,
	parametros, prioridad_consumo, unidad_habil, pagable_en_finiquito, vigente_desde`

func escanearTipo(row pgx.Row) (domain.TipoDeVacacion, error) {
	var t domain.TipoDeVacacion
	var params []byte

	err := row.Scan(&t.ID, &t.EmpresaID, &t.Codigo, &t.Nombre,
		&t.PoliticaDevengo, &t.PoliticaVencimiento, &params,
		&t.PrioridadConsumo, &t.UnidadHabil, &t.PagableEnFiniquito, &t.VigenteDesde)
	if err != nil {
		return domain.TipoDeVacacion{}, err
	}
	if err := json.Unmarshal(params, &t.Parametros); err != nil {
		return domain.TipoDeVacacion{}, err
	}
	return t, nil
}

// ListarTipos devuelve los tipos de vacación de la empresa, ordenados por la
// prioridad con que se consumen.
func ListarTipos(ctx context.Context, q Querier, empresaID uuid.UUID) ([]domain.TipoDeVacacion, error) {
	rows, err := q.Query(ctx,
		`SELECT `+camposTipo+` FROM tipo_de_vacacion WHERE empresa_id = $1
		 ORDER BY prioridad_consumo, codigo`, empresaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.TipoDeVacacion
	for rows.Next() {
		t, err := escanearTipo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TiposPorID devuelve los tipos indexados por su id, que es la forma en que los
// consume el dominio.
func TiposPorID(ctx context.Context, q Querier, empresaID uuid.UUID) (map[uuid.UUID]domain.TipoDeVacacion, error) {
	tipos, err := ListarTipos(ctx, q, empresaID)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]domain.TipoDeVacacion, len(tipos))
	for _, t := range tipos {
		out[t.ID] = t
	}
	return out, nil
}

// TipoPorCodigo busca un tipo por su código de negocio.
func TipoPorCodigo(ctx context.Context, q Querier, empresaID uuid.UUID, codigo string) (domain.TipoDeVacacion, error) {
	row := q.QueryRow(ctx,
		`SELECT `+camposTipo+` FROM tipo_de_vacacion WHERE empresa_id = $1 AND codigo = $2`,
		empresaID, codigo)
	return escanearTipo(row)
}

// CrearTipo inserta un tipo nuevo. Es lo que hace posible el Requisito 7:
// agregar "días por rendimiento" es esta llamada, no un despliegue.
func CrearTipo(ctx context.Context, q Querier, t domain.TipoDeVacacion) (uuid.UUID, error) {
	params, err := json.Marshal(t.Parametros)
	if err != nil {
		return uuid.Nil, err
	}

	var id uuid.UUID
	err = q.QueryRow(ctx, `
		INSERT INTO tipo_de_vacacion (empresa_id, codigo, nombre, politica_devengo,
			politica_vencimiento, parametros, prioridad_consumo, unidad_habil,
			pagable_en_finiquito, vigente_desde)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		t.EmpresaID, t.Codigo, t.Nombre, t.PoliticaDevengo, t.PoliticaVencimiento,
		params, t.PrioridadConsumo, t.UnidadHabil, t.PagableEnFiniquito, t.VigenteDesde,
	).Scan(&id)
	return id, err
}

// ActualizarTipo modifica la configuración de un tipo existente.
func ActualizarTipo(ctx context.Context, q Querier, t domain.TipoDeVacacion) error {
	params, err := json.Marshal(t.Parametros)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		UPDATE tipo_de_vacacion
		SET nombre = $3, politica_devengo = $4, politica_vencimiento = $5,
			parametros = $6, prioridad_consumo = $7, unidad_habil = $8,
			pagable_en_finiquito = $9
		WHERE empresa_id = $1 AND id = $2`,
		t.EmpresaID, t.ID, t.Nombre, t.PoliticaDevengo, t.PoliticaVencimiento,
		params, t.PrioridadConsumo, t.UnidadHabil, t.PagableEnFiniquito)
	return err
}
```

- [ ] **Step 4: Crear `internal/store/ledger_repo.go`**

```go
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// BolsasDeColaborador arma las bolsas de un colaborador: cada otorgamiento con
// todos sus movimientos. El saldo NO se lee de ninguna columna; se calcula
// después sumando estos movimientos.
//
// Si tipoID no es nil, filtra a las bolsas de ese tipo.
func BolsasDeColaborador(
	ctx context.Context, q Querier, empresaID, colaboradorID uuid.UUID, tipoID *uuid.UUID,
) ([]domain.Bolsa, error) {
	rows, err := q.Query(ctx, `
		SELECT o.id, o.empresa_id, o.colaborador_id, o.tipo_id, o.periodo_desde,
		       o.periodo_hasta, o.dias_otorgados::text, o.devengado_el, o.vence_el,
		       o.origen, t.prioridad_consumo
		FROM otorgamiento o
		JOIN tipo_de_vacacion t ON t.id = o.tipo_id
		WHERE o.empresa_id = $1
		  AND o.colaborador_id = $2
		  AND ($3::uuid IS NULL OR o.tipo_id = $3)
		ORDER BY o.devengado_el`,
		empresaID, colaboradorID, tipoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bolsas := make([]domain.Bolsa, 0)
	indice := make(map[uuid.UUID]int)

	for rows.Next() {
		var o domain.Otorgamiento
		var dias string
		var prioridad int

		if err := rows.Scan(&o.ID, &o.EmpresaID, &o.ColaboradorID, &o.TipoID,
			&o.PeriodoDesde, &o.PeriodoHasta, &dias, &o.DevengadoEl, &o.VenceEl,
			&o.Origen, &prioridad); err != nil {
			return nil, err
		}
		o.DiasOtorgados = decimal.RequireFromString(dias)

		indice[o.ID] = len(bolsas)
		bolsas = append(bolsas, domain.Bolsa{Otorgamiento: o, Prioridad: prioridad})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(bolsas) == 0 {
		return bolsas, nil
	}

	ids := make([]uuid.UUID, 0, len(bolsas))
	for _, b := range bolsas {
		ids = append(ids, b.Otorgamiento.ID)
	}

	movs, err := q.Query(ctx, `
		SELECT id, empresa_id, otorgamiento_id, solicitud_id, cantidad::text, clase,
		       fecha_efectiva, fecha_registro, actor_id, motivo, clave_idempotencia, reversa_de
		FROM movimiento
		WHERE otorgamiento_id = ANY($1)
		ORDER BY fecha_efectiva, fecha_registro`, ids)
	if err != nil {
		return nil, err
	}
	defer movs.Close()

	for movs.Next() {
		var m domain.Movimiento
		var cantidad string

		if err := movs.Scan(&m.ID, &m.EmpresaID, &m.OtorgamientoID, &m.SolicitudID,
			&cantidad, &m.Clase, &m.FechaEfectiva, &m.FechaRegistro, &m.ActorID,
			&m.Motivo, &m.ClaveIdempotencia, &m.ReversaDe); err != nil {
			return nil, err
		}
		m.Cantidad = decimal.RequireFromString(cantidad)

		if i, ok := indice[m.OtorgamientoID]; ok {
			bolsas[i].Movimientos = append(bolsas[i].Movimientos, m)
		}
	}
	return bolsas, movs.Err()
}

// MovimientoConContexto es una fila del historial tal como se muestra en la
// interfaz: el movimiento más el tipo y el actor, resueltos.
type MovimientoConContexto struct {
	domain.Movimiento
	TipoCodigo  string
	TipoNombre  string
	ActorNombre string
	VenceEl     *time.Time
}

// HistorialDeColaborador devuelve el ledger completo del colaborador, en orden
// cronológico. Con `hasta` distinto de nil, corta por fecha efectiva: eso es lo
// que permite reconstruir y explicar el saldo a cualquier fecha pasada.
func HistorialDeColaborador(
	ctx context.Context, q Querier, empresaID, colaboradorID uuid.UUID, hasta *time.Time,
) ([]MovimientoConContexto, error) {
	rows, err := q.Query(ctx, `
		SELECT m.id, m.empresa_id, m.otorgamiento_id, m.solicitud_id, m.cantidad::text,
		       m.clase, m.fecha_efectiva, m.fecha_registro, m.actor_id, m.motivo,
		       m.clave_idempotencia, m.reversa_de,
		       t.codigo, t.nombre, a.nombre, o.vence_el
		FROM movimiento m
		JOIN otorgamiento o ON o.id = m.otorgamiento_id
		JOIN tipo_de_vacacion t ON t.id = o.tipo_id
		JOIN colaborador a ON a.id = m.actor_id
		WHERE m.empresa_id = $1
		  AND o.colaborador_id = $2
		  AND ($3::date IS NULL OR m.fecha_efectiva <= $3)
		ORDER BY m.fecha_efectiva, m.fecha_registro`,
		empresaID, colaboradorID, hasta)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MovimientoConContexto
	for rows.Next() {
		var m MovimientoConContexto
		var cantidad string

		if err := rows.Scan(&m.ID, &m.EmpresaID, &m.OtorgamientoID, &m.SolicitudID,
			&cantidad, &m.Clase, &m.FechaEfectiva, &m.FechaRegistro, &m.ActorID,
			&m.Motivo, &m.ClaveIdempotencia, &m.ReversaDe,
			&m.TipoCodigo, &m.TipoNombre, &m.ActorNombre, &m.VenceEl); err != nil {
			return nil, err
		}
		m.Cantidad = decimal.RequireFromString(cantidad)
		out = append(out, m)
	}
	return out, rows.Err()
}

// CrearOtorgamiento inserta una bolsa nueva.
func CrearOtorgamiento(ctx context.Context, q Querier, o domain.Otorgamiento) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO otorgamiento (empresa_id, colaborador_id, tipo_id, periodo_desde,
			periodo_hasta, dias_otorgados, devengado_el, vence_el, origen)
		VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $8, $9)
		RETURNING id`,
		o.EmpresaID, o.ColaboradorID, o.TipoID, o.PeriodoDesde, o.PeriodoHasta,
		o.DiasOtorgados.String(), o.DevengadoEl, o.VenceEl, o.Origen,
	).Scan(&id)
	return id, err
}

// InsertarMovimiento escribe una fila en el ledger. Devuelve false si la clave
// de idempotencia ya existía, en cuyo caso NO se insertó nada.
//
// Esto es lo que hace que reejecutar un job sea seguro: la segunda corrida
// colisiona contra el índice único y no duplica movimientos.
func InsertarMovimiento(ctx context.Context, q Querier, m domain.Movimiento) (bool, error) {
	tag, err := q.Exec(ctx, `
		INSERT INTO movimiento (empresa_id, otorgamiento_id, solicitud_id, cantidad,
			clase, fecha_efectiva, actor_id, motivo, clave_idempotencia, reversa_de)
		VALUES ($1, $2, $3, $4::numeric, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (clave_idempotencia) DO NOTHING`,
		m.EmpresaID, m.OtorgamientoID, m.SolicitudID, m.Cantidad.String(),
		m.Clase, m.FechaEfectiva, m.ActorID, m.Motivo, m.ClaveIdempotencia, m.ReversaDe)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
```

- [ ] **Step 5: Crear `internal/store/solicitud_repo.go`**

```go
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// Solicitud es una solicitud de vacaciones con los datos que necesita la
// interfaz ya resueltos.
type Solicitud struct {
	ID             uuid.UUID
	EmpresaID      uuid.UUID
	ColaboradorID  uuid.UUID
	ColaboradorNom string
	TipoID         uuid.UUID
	TipoCodigo     string
	Desde          time.Time
	Hasta          time.Time
	DiasHabiles    decimal.Decimal
	Estado         domain.EstadoSolicitud
	AprobadorID    *uuid.UUID
	DecididoEl     *time.Time
	CreadaEl       time.Time
}

const selectSolicitud = `
	SELECT s.id, s.empresa_id, s.colaborador_id, c.nombre, s.tipo_id, t.codigo,
	       s.desde, s.hasta, s.dias_habiles::text, s.estado, s.aprobador_id,
	       s.decidido_el, s.creada_el
	FROM solicitud_de_vacaciones s
	JOIN colaborador c ON c.id = s.colaborador_id
	JOIN tipo_de_vacacion t ON t.id = s.tipo_id`

func escanearSolicitud(row pgx.Row) (Solicitud, error) {
	var s Solicitud
	var dias string
	err := row.Scan(&s.ID, &s.EmpresaID, &s.ColaboradorID, &s.ColaboradorNom,
		&s.TipoID, &s.TipoCodigo, &s.Desde, &s.Hasta, &dias, &s.Estado,
		&s.AprobadorID, &s.DecididoEl, &s.CreadaEl)
	if err != nil {
		return Solicitud{}, err
	}
	s.DiasHabiles = decimal.RequireFromString(dias)
	return s, nil
}

// CrearSolicitud inserta una solicitud en estado PENDIENTE.
func CrearSolicitud(ctx context.Context, q Querier, s Solicitud) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx, `
		INSERT INTO solicitud_de_vacaciones (empresa_id, colaborador_id, tipo_id,
			desde, hasta, dias_habiles, estado)
		VALUES ($1, $2, $3, $4, $5, $6::numeric, $7)
		RETURNING id`,
		s.EmpresaID, s.ColaboradorID, s.TipoID, s.Desde, s.Hasta,
		s.DiasHabiles.String(), domain.EstadoPendiente,
	).Scan(&id)
	return id, err
}

// SolicitudPorID busca una solicitud dentro de su empresa.
func SolicitudPorID(ctx context.Context, q Querier, empresaID, id uuid.UUID) (Solicitud, error) {
	row := q.QueryRow(ctx, selectSolicitud+` WHERE s.empresa_id = $1 AND s.id = $2`, empresaID, id)
	return escanearSolicitud(row)
}

// ListarSolicitudes devuelve las solicitudes de la empresa. Con colaboradorID
// distinto de nil, solo las de ese colaborador.
func ListarSolicitudes(
	ctx context.Context, q Querier, empresaID uuid.UUID, colaboradorID *uuid.UUID,
) ([]Solicitud, error) {
	rows, err := q.Query(ctx, selectSolicitud+`
		WHERE s.empresa_id = $1
		  AND ($2::uuid IS NULL OR s.colaborador_id = $2)
		ORDER BY s.creada_el DESC`, empresaID, colaboradorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Solicitud
	for rows.Next() {
		s, err := escanearSolicitud(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DecidirSolicitud cambia el estado de una solicitud pendiente. La condición
// sobre el estado actual es la que impide aprobar dos veces la misma solicitud.
func DecidirSolicitud(
	ctx context.Context, q Querier, id uuid.UUID,
	estado domain.EstadoSolicitud, aprobadorID uuid.UUID, cuando time.Time,
) (bool, error) {
	tag, err := q.Exec(ctx, `
		UPDATE solicitud_de_vacaciones
		SET estado = $2, aprobador_id = $3, decidido_el = $4
		WHERE id = $1 AND estado = 'PENDIENTE'`,
		id, estado, aprobadorID, cuando)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DiasPendientesPorTipo suma los días comprometidos en solicitudes que todavía
// no se aprueban ni se rechazan.
//
// Estos días NO están en el ledger: el ledger solo registra hechos consumados.
// Pero se descuentan del disponible que se ofrece para solicitar, de modo que
// un colaborador no pueda comprometer dos veces los mismos días.
func DiasPendientesPorTipo(
	ctx context.Context, q Querier, empresaID, colaboradorID uuid.UUID,
) (map[uuid.UUID]decimal.Decimal, error) {
	rows, err := q.Query(ctx, `
		SELECT tipo_id, SUM(dias_habiles)::text
		FROM solicitud_de_vacaciones
		WHERE empresa_id = $1 AND colaborador_id = $2 AND estado = 'PENDIENTE'
		GROUP BY tipo_id`, empresaID, colaboradorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]decimal.Decimal)
	for rows.Next() {
		var tipoID uuid.UUID
		var suma string
		if err := rows.Scan(&tipoID, &suma); err != nil {
			return nil, err
		}
		out[tipoID] = decimal.RequireFromString(suma)
	}
	return out, rows.Err()
}
```

- [ ] **Step 6: Crear `internal/store/calendario_repo.go`**

```go
package store

import (
	"context"
	"time"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// CargarCalendario trae las fechas inhábiles de un ámbito. Se carga completo:
// son unos cientos de filas y se consulta en cada cálculo de días hábiles.
func CargarCalendario(ctx context.Context, q Querier, ambito string) ([]time.Time, error) {
	rows, err := q.Query(ctx,
		`SELECT fecha FROM calendario_laboral WHERE ambito = $1 ORDER BY fecha`, ambito)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var f time.Time
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append(out, domain.SoloFecha(f))
	}
	return out, rows.Err()
}

// Feriado es una fila del calendario laboral.
type Feriado struct {
	Fecha  time.Time
	Ambito string
	Tipo   string
	Nombre string
}

// InsertarFeriados carga el calendario. Solo lo usa la semilla. Es idempotente:
// reejecutarla no duplica fechas.
func InsertarFeriados(ctx context.Context, q Querier, feriados []Feriado) error {
	for _, f := range feriados {
		_, err := q.Exec(ctx, `
			INSERT INTO calendario_laboral (fecha, ambito, tipo, nombre)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (fecha, ambito) DO NOTHING`,
			f.Fecha, f.Ambito, f.Tipo, f.Nombre)
		if err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 7: Verificar que todo compila**

Run: `go build ./...`
Expected: sin salida (éxito).

- [ ] **Step 8: Commit**

```bash
git add internal/store
git commit -m "feat(store): repositorios con filtro por empresa y decimales sin float"
```

---

## Task 14: Servicio de saldo

**Files:**
- Create: `internal/app/app.go`, `internal/app/saldo.go`

- [ ] **Step 1: Crear `internal/app/app.go`**

```go
package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// Servicio agrupa los casos de uso. Recibe el pool y el id de la empresa, que
// en este MVP es única pero viaja explícito en cada consulta.
type Servicio struct {
	Pool      *pgxpool.Pool
	EmpresaID uuid.UUID
	// Hoy permite fijar la fecha actual desde afuera. En producción es
	// time.Now; en tests y en la semilla se sustituye para poder recorrer la
	// historia de un colaborador.
	Hoy func() time.Time
}

// NuevoServicio construye el servicio con el reloj real.
func NuevoServicio(pool *pgxpool.Pool, empresaID uuid.UUID) *Servicio {
	return &Servicio{
		Pool:      pool,
		EmpresaID: empresaID,
		Hoy:       func() time.Time { return domain.SoloFecha(time.Now().UTC()) },
	}
}

// Calendario carga el calendario laboral vigente.
func (s *Servicio) Calendario(ctx context.Context) (*domain.Calendario, error) {
	fechas, err := store.CargarCalendario(ctx, s.Pool, "CL")
	if err != nil {
		return nil, err
	}
	return domain.NuevoCalendario(fechas), nil
}
```

- [ ] **Step 2: Crear `internal/app/saldo.go`**

```go
package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// SaldoDeTipo es el saldo de un tipo tal como lo consume la interfaz.
type SaldoDeTipo struct {
	TipoID     uuid.UUID              `json:"tipo_id"`
	TipoCodigo string                 `json:"tipo_codigo"`
	TipoNombre string                 `json:"tipo_nombre"`
	Disponible decimal.Decimal        `json:"disponible"`
	Pendiente  decimal.Decimal        `json:"pendiente"`
	Solicitable decimal.Decimal       `json:"solicitable"`
	PorVencer  []domain.BolsaPorVencer `json:"por_vencer"`
}

// SaldoCompleto es la respuesta del endpoint de saldo. Los campos reservados a
// RRHH van como punteros: van en nil para un colaborador y el JSON los omite.
type SaldoCompleto struct {
	ColaboradorID     uuid.UUID       `json:"colaborador_id"`
	ColaboradorNombre string          `json:"colaborador_nombre"`
	FechaIngreso      time.Time       `json:"fecha_ingreso"`
	AlDia             time.Time       `json:"al_dia"`
	Total             decimal.Decimal `json:"total_disponible"`
	PorTipo           []SaldoDeTipo   `json:"por_tipo"`

	// Solo RRHH. El colaborador ve únicamente su disponible: así lo define el
	// Requisito 6.
	DevengadoNoOtorgado *decimal.Decimal      `json:"devengado_no_otorgado,omitempty"`
	Proporcional        *domain.Proporcional  `json:"proporcional,omitempty"`
}

// Saldo proyecta el saldo de un colaborador sumando su ledger.
//
// El saldo no se lee de ninguna tabla. Se calcula acá, en cada consulta, a
// partir de los movimientos. Esa es la decisión central del sistema: un número
// que no existe no puede quedar desactualizado.
func (s *Servicio) Saldo(ctx context.Context, colaboradorID uuid.UUID, verComoRRHH bool) (SaldoCompleto, error) {
	hoy := s.Hoy()

	colaborador, err := store.ColaboradorPorID(ctx, s.Pool, s.EmpresaID, colaboradorID)
	if err != nil {
		return SaldoCompleto{}, err
	}
	bolsas, err := store.BolsasDeColaborador(ctx, s.Pool, s.EmpresaID, colaboradorID, nil)
	if err != nil {
		return SaldoCompleto{}, err
	}
	tipos, err := store.TiposPorID(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return SaldoCompleto{}, err
	}
	pendientes, err := store.DiasPendientesPorTipo(ctx, s.Pool, s.EmpresaID, colaboradorID)
	if err != nil {
		return SaldoCompleto{}, err
	}

	proyeccion := domain.ProyectarSaldo(bolsas, tipos, hoy)

	out := SaldoCompleto{
		ColaboradorID:     colaborador.ID,
		ColaboradorNombre: colaborador.Nombre,
		FechaIngreso:      colaborador.FechaIngreso,
		AlDia:             hoy,
		Total:             proyeccion.Total(),
	}

	for _, t := range proyeccion.PorTipo {
		pendiente, existe := pendientes[t.TipoID]
		if !existe {
			pendiente = decimal.Zero
		}
		out.PorTipo = append(out.PorTipo, SaldoDeTipo{
			TipoID:      t.TipoID,
			TipoCodigo:  t.TipoCodigo,
			TipoNombre:  t.TipoNombre,
			Disponible:  t.Disponible,
			Pendiente:   pendiente,
			Solicitable: t.Disponible.Sub(pendiente),
			PorVencer:   t.PorVencer,
		})
	}

	if verComoRRHH {
		proporcional := domain.CalcularProporcional(colaborador, hoy)
		out.DevengadoNoOtorgado = &proporcional.Dias
		out.Proporcional = &proporcional
	}

	return out, nil
}

// Finiquito responde cuánto se le debe a un colaborador si se desvincula en la
// fecha dada. Es solo lectura: no escribe ningún movimiento.
func (s *Servicio) Finiquito(ctx context.Context, colaboradorID uuid.UUID, fecha time.Time) (domain.Finiquito, error) {
	colaborador, err := store.ColaboradorPorID(ctx, s.Pool, s.EmpresaID, colaboradorID)
	if err != nil {
		return domain.Finiquito{}, err
	}
	bolsas, err := store.BolsasDeColaborador(ctx, s.Pool, s.EmpresaID, colaboradorID, nil)
	if err != nil {
		return domain.Finiquito{}, err
	}
	tipos, err := store.TiposPorID(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return domain.Finiquito{}, err
	}

	pagables := make(map[uuid.UUID]bool, len(tipos))
	for id, t := range tipos {
		pagables[id] = t.PagableEnFiniquito
	}

	return domain.CalcularFiniquito(colaborador, bolsas, pagables, fecha), nil
}

// Historial devuelve el ledger de un colaborador, opcionalmente cortado a una
// fecha pasada.
func (s *Servicio) Historial(
	ctx context.Context, colaboradorID uuid.UUID, hasta *time.Time,
) ([]store.MovimientoConContexto, error) {
	return store.HistorialDeColaborador(ctx, s.Pool, s.EmpresaID, colaboradorID, hasta)
}
```

- [ ] **Step 3: Verificar que compila**

Run: `go build ./...`
Expected: sin salida.

- [ ] **Step 4: Commit**

```bash
git add internal/app
git commit -m "feat(app): proyección de saldo, finiquito e historial"
```

---

## Task 15: Jobs de devengo y vencimiento

Requisitos 1, 2 y 8. Sin interfaz: se exponen como endpoints y se explican verbalmente.

**Files:**
- Create: `internal/app/jobs.go`

- [ ] **Step 1: Crear `internal/app/jobs.go`**

```go
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// ResultadoJob resume lo que hizo una corrida.
type ResultadoJob struct {
	Fecha       time.Time `json:"fecha"`
	Evaluados   int       `json:"evaluados"`
	Creados     int       `json:"creados"`
	YaExistian  int       `json:"ya_existian"`
	Detalle     []string  `json:"detalle"`
}

// CorrerDevengo evalúa a todos los colaboradores contra todos los tipos con
// devengo automático y crea los otorgamientos que correspondan a esa fecha.
//
// Es idempotente y acepta fecha objetivo. Reejecutarlo para una fecha ya
// procesada no duplica nada: la clave de idempotencia colisiona. Correrlo con
// una fecha pasada recupera un día que no se procesó. Esas dos propiedades son
// las que convierten una falla silenciosa en algo recuperable.
func (s *Servicio) CorrerDevengo(ctx context.Context, fecha time.Time, actorID uuid.UUID) (ResultadoJob, error) {
	fecha = domain.SoloFecha(fecha)
	res := ResultadoJob{Fecha: fecha}

	colaboradores, err := store.ListarColaboradores(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return res, err
	}
	tipos, err := store.ListarTipos(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return res, err
	}

	for _, c := range colaboradores {
		if c.FechaTermino != nil && !fecha.Before(*c.FechaTermino) {
			continue
		}
		for _, tipo := range tipos {
			res.Evaluados++

			resultado, hubo := domain.Devengar(tipo, c, fecha)
			if !hubo {
				continue
			}

			creado, err := s.otorgar(ctx, c, tipo, resultado, domain.OrigenAutomatico, actorID, resultado.Detalle)
			if err != nil {
				return res, fmt.Errorf("devengando %s/%s: %w", c.Nombre, tipo.Codigo, err)
			}
			if creado {
				res.Creados++
				res.Detalle = append(res.Detalle,
					fmt.Sprintf("%s · %s · %s días", c.Nombre, tipo.Codigo, resultado.Dias))
			} else {
				res.YaExistian++
			}
		}
	}
	return res, nil
}

// otorgar crea la bolsa y su movimiento ACCRUAL en una transacción. Devuelve
// false si el movimiento ya existía por clave de idempotencia, en cuyo caso la
// transacción se revierte y no queda una bolsa huérfana.
func (s *Servicio) otorgar(
	ctx context.Context,
	c domain.Colaborador,
	tipo domain.TipoDeVacacion,
	resultado domain.ResultadoDevengo,
	origen domain.Origen,
	actorID uuid.UUID,
	motivo string,
) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	otorgamiento := domain.Otorgamiento{
		EmpresaID:     s.EmpresaID,
		ColaboradorID: c.ID,
		TipoID:        tipo.ID,
		PeriodoDesde:  resultado.PeriodoDesde,
		PeriodoHasta:  resultado.PeriodoHasta,
		DiasOtorgados: resultado.Dias,
		DevengadoEl:   resultado.DevengadoEl,
		VenceEl:       domain.CalcularVencimiento(tipo, resultado.DevengadoEl),
		Origen:        origen,
	}

	otorgamientoID, err := store.CrearOtorgamiento(ctx, tx, otorgamiento)
	if err != nil {
		return false, err
	}

	clave := domain.ClaveIdempotencia(domain.ClaseAccrual, c.ID, tipo.ID, resultado.PeriodoDesde)
	if origen == domain.OrigenManual {
		// Un otorgamiento manual puede repetirse legítimamente el mismo día,
		// así que su clave incluye el id de la bolsa recién creada.
		clave = domain.ClaveIdempotenciaBolsa(domain.ClaseAccrual, otorgamientoID, resultado.DevengadoEl)
	}

	insertado, err := store.InsertarMovimiento(ctx, tx, domain.Movimiento{
		EmpresaID:         s.EmpresaID,
		OtorgamientoID:    otorgamientoID,
		Cantidad:          resultado.Dias,
		Clase:             domain.ClaseAccrual,
		FechaEfectiva:     resultado.DevengadoEl,
		ActorID:           actorID,
		Motivo:            motivo,
		ClaveIdempotencia: clave,
	})
	if err != nil {
		return false, err
	}
	if !insertado {
		// El otorgamiento ya existía: revertimos para no dejar una bolsa vacía.
		return false, nil
	}

	return true, tx.Commit(ctx)
}

// CorrerVencimiento hace vencer las bolsas cuya fecha de vencimiento ya llegó,
// emitiendo un EXPIRATION por el remanente exacto de cada una.
//
// También es idempotente: la clave incluye la bolsa y su fecha de vencimiento.
func (s *Servicio) CorrerVencimiento(ctx context.Context, fecha time.Time, actorID uuid.UUID) (ResultadoJob, error) {
	fecha = domain.SoloFecha(fecha)
	res := ResultadoJob{Fecha: fecha}

	colaboradores, err := store.ListarColaboradores(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return res, err
	}

	for _, c := range colaboradores {
		bolsas, err := store.BolsasDeColaborador(ctx, s.Pool, s.EmpresaID, c.ID, nil)
		if err != nil {
			return res, err
		}

		for _, b := range bolsas {
			res.Evaluados++

			vence := b.Otorgamiento.VenceEl
			if vence == nil || fecha.Before(*vence) {
				continue
			}
			remanente := b.Remanente()
			if !remanente.IsPositive() {
				continue
			}

			insertado, err := store.InsertarMovimiento(ctx, s.Pool, domain.Movimiento{
				EmpresaID:      s.EmpresaID,
				OtorgamientoID: b.Otorgamiento.ID,
				Cantidad:       remanente.Neg(),
				Clase:          domain.ClaseExpiration,
				FechaEfectiva:  *vence,
				ActorID:        actorID,
				Motivo: fmt.Sprintf("vencimiento automático: %s días no utilizados del período %s",
					remanente, b.Otorgamiento.PeriodoDesde.Format("2006-01-02")),
				ClaveIdempotencia: domain.ClaveIdempotenciaBolsa(
					domain.ClaseExpiration, b.Otorgamiento.ID, *vence),
			})
			if err != nil {
				return res, err
			}
			if insertado {
				res.Creados++
				res.Detalle = append(res.Detalle,
					fmt.Sprintf("%s · %s días vencidos el %s", c.Nombre, remanente, vence.Format("2006-01-02")))
			} else {
				res.YaExistian++
			}
		}
	}
	return res, nil
}
```

- [ ] **Step 2: Verificar que compila**

Run: `go build ./...`
Expected: sin salida.

- [ ] **Step 3: Commit**

```bash
git add internal/app/jobs.go
git commit -m "feat(app): jobs idempotentes de devengo y vencimiento con fecha objetivo"
```

---

## Task 16: Solicitudes — crear, aprobar y rechazar

Requisitos 4 y 9. La aprobación es el único lugar del sistema donde se escriben movimientos de consumo, y ocurre dentro de una transacción con bloqueo.

**Files:**
- Create: `internal/app/solicitudes.go`

- [ ] **Step 1: Crear `internal/app/solicitudes.go`**

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

var (
	// ErrRangoInvalido indica que el rango pedido no contiene días hábiles.
	ErrRangoInvalido = errors.New("el rango no contiene días hábiles")
	// ErrYaDecidida indica que la solicitud ya no está pendiente.
	ErrYaDecidida = errors.New("la solicitud ya fue decidida")
)

// PreviewSolicitud muestra cuántos días hábiles descuenta un rango antes de que
// el colaborador confirme.
type PreviewSolicitud struct {
	Desde       time.Time       `json:"desde"`
	Hasta       time.Time       `json:"hasta"`
	DiasHabiles decimal.Decimal `json:"dias_habiles"`
	DiasCorridos int            `json:"dias_corridos"`
}

// PreviewDiasHabiles cuenta los días hábiles de un rango sin crear nada.
func (s *Servicio) PreviewDiasHabiles(ctx context.Context, desde, hasta time.Time) (PreviewSolicitud, error) {
	cal, err := s.Calendario(ctx)
	if err != nil {
		return PreviewSolicitud{}, err
	}
	habiles := cal.ContarHabiles(desde, hasta)

	return PreviewSolicitud{
		Desde:        domain.SoloFecha(desde),
		Hasta:        domain.SoloFecha(hasta),
		DiasHabiles:  decimal.NewFromInt(int64(habiles)),
		DiasCorridos: domain.DiasEntre(desde, hasta) + 1,
	}, nil
}

// CrearSolicitud registra la intención del colaborador. NO escribe movimientos:
// el ledger solo registra hechos consumados, y una solicitud pendiente todavía
// no lo es. Sí valida que el saldo alcance descontando lo ya comprometido en
// otras solicitudes pendientes.
func (s *Servicio) CrearSolicitud(
	ctx context.Context, colaboradorID, tipoID uuid.UUID, desde, hasta time.Time,
) (store.Solicitud, error) {
	preview, err := s.PreviewDiasHabiles(ctx, desde, hasta)
	if err != nil {
		return store.Solicitud{}, err
	}
	if !preview.DiasHabiles.IsPositive() {
		return store.Solicitud{}, ErrRangoInvalido
	}

	saldo, err := s.Saldo(ctx, colaboradorID, false)
	if err != nil {
		return store.Solicitud{}, err
	}

	solicitable := decimal.Zero
	for _, t := range saldo.PorTipo {
		if t.TipoID == tipoID {
			solicitable = t.Solicitable
		}
	}
	if preview.DiasHabiles.GreaterThan(solicitable) {
		return store.Solicitud{}, fmt.Errorf(
			"%w: pide %s días hábiles y tiene %s solicitables",
			domain.ErrSaldoInsuficiente, preview.DiasHabiles, solicitable)
	}

	id, err := store.CrearSolicitud(ctx, s.Pool, store.Solicitud{
		EmpresaID:     s.EmpresaID,
		ColaboradorID: colaboradorID,
		TipoID:        tipoID,
		Desde:         preview.Desde,
		Hasta:         preview.Hasta,
		DiasHabiles:   preview.DiasHabiles,
	})
	if err != nil {
		return store.Solicitud{}, err
	}
	return store.SolicitudPorID(ctx, s.Pool, s.EmpresaID, id)
}

// AprobarSolicitud es el único punto del sistema que escribe consumos.
//
// Todo ocurre en una transacción con bloqueo de fila sobre el colaborador: el
// bloqueo impide que dos aprobaciones concurrentes lean el mismo saldo y gasten
// los mismos días. Dentro, el asignador reparte FIFO entre las bolsas del tipo
// pedido y cada tramo genera su propio movimiento, de modo que el historial
// muestra de qué bolsa salió cada día.
func (s *Servicio) AprobarSolicitud(ctx context.Context, solicitudID, aprobadorID uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	solicitud, err := store.SolicitudPorID(ctx, tx, s.EmpresaID, solicitudID)
	if err != nil {
		return err
	}
	if solicitud.Estado != domain.EstadoPendiente {
		return ErrYaDecidida
	}

	if err := store.BloquearColaborador(ctx, tx, solicitud.ColaboradorID); err != nil {
		return err
	}

	bolsas, err := store.BolsasDeColaborador(ctx, tx, s.EmpresaID, solicitud.ColaboradorID, &solicitud.TipoID)
	if err != nil {
		return err
	}

	asignaciones, err := domain.AsignarConsumo(bolsas, solicitud.DiasHabiles, s.Hoy())
	if err != nil {
		return err
	}

	for i, a := range asignaciones {
		_, err := store.InsertarMovimiento(ctx, tx, domain.Movimiento{
			EmpresaID:      s.EmpresaID,
			OtorgamientoID: a.OtorgamientoID,
			SolicitudID:    &solicitud.ID,
			Cantidad:       a.Dias.Neg(),
			Clase:          domain.ClaseConsumption,
			FechaEfectiva:  solicitud.Desde,
			ActorID:        aprobadorID,
			Motivo: fmt.Sprintf("vacaciones %s al %s",
				solicitud.Desde.Format("2006-01-02"), solicitud.Hasta.Format("2006-01-02")),
			// El índice del tramo hace única la clave cuando una misma
			// solicitud se reparte entre varias bolsas.
			ClaveIdempotencia: fmt.Sprintf("CONSUMPTION:%s:%d", solicitud.ID, i),
		})
		if err != nil {
			return err
		}
	}

	decidida, err := store.DecidirSolicitud(ctx, tx, solicitud.ID,
		domain.EstadoAprobada, aprobadorID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !decidida {
		return ErrYaDecidida
	}

	return tx.Commit(ctx)
}

// RechazarSolicitud cierra la solicitud sin tocar el ledger. Los días
// comprometidos vuelven a estar solicitables automáticamente, porque el
// disponible solicitable se calcula descontando solo las pendientes.
func (s *Servicio) RechazarSolicitud(ctx context.Context, solicitudID, aprobadorID uuid.UUID) error {
	decidida, err := store.DecidirSolicitud(ctx, s.Pool, solicitudID,
		domain.EstadoRechazada, aprobadorID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !decidida {
		return ErrYaDecidida
	}
	return nil
}

// ListarSolicitudes devuelve las solicitudes visibles para quien consulta.
func (s *Servicio) ListarSolicitudes(ctx context.Context, actor domain.Colaborador) ([]store.Solicitud, error) {
	if actor.EsRRHH() {
		return store.ListarSolicitudes(ctx, s.Pool, s.EmpresaID, nil)
	}
	return store.ListarSolicitudes(ctx, s.Pool, s.EmpresaID, &actor.ID)
}
```

- [ ] **Step 2: Verificar que compila**

Run: `go build ./...`
Expected: sin salida.

- [ ] **Step 3: Commit**

```bash
git add internal/app/solicitudes.go
git commit -m "feat(app): solicitudes con aprobación transaccional y consumo FIFO"
```

---

## Task 17: Otorgamiento manual y ABM de tipos

Requisito 7: RRHH define tipos nuevos y otorga días manualmente, sin desarrollo ni despliegue.

**Files:**
- Create: `internal/app/rrhh.go`

- [ ] **Step 1: Crear `internal/app/rrhh.go`**

```go
package app

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// ErrMotivoRequerido indica que se intentó otorgar días sin justificación.
var ErrMotivoRequerido = errors.New("el motivo es obligatorio")

// OtorgarManual carga un saldo especial a un colaborador. El motivo es
// obligatorio: un movimiento del ledger sin explicación es exactamente el
// problema que este sistema viene a resolver.
func (s *Servicio) OtorgarManual(
	ctx context.Context,
	colaboradorID, tipoID uuid.UUID,
	dias decimal.Decimal,
	motivo string,
	actorID uuid.UUID,
) error {
	if motivo == "" {
		return ErrMotivoRequerido
	}
	if !dias.IsPositive() {
		return errors.New("los días otorgados deben ser positivos")
	}

	hoy := s.Hoy()

	colaborador, err := store.ColaboradorPorID(ctx, s.Pool, s.EmpresaID, colaboradorID)
	if err != nil {
		return err
	}
	tipos, err := store.TiposPorID(ctx, s.Pool, s.EmpresaID)
	if err != nil {
		return err
	}
	tipo, existe := tipos[tipoID]
	if !existe {
		return errors.New("tipo de vacación desconocido")
	}

	// El período de una carga manual es el año en curso desde la fecha de carga.
	resultado := domain.ResultadoDevengo{
		Dias:         dias,
		DiasBase:     dias,
		PeriodoDesde: hoy,
		PeriodoHasta: hoy.AddDate(1, 0, -1),
		DevengadoEl:  hoy,
		Detalle:      motivo,
	}

	_, err = s.otorgar(ctx, colaborador, tipo, resultado, domain.OrigenManual, actorID, motivo)
	return err
}

// CrearTipo registra un tipo de vacación nuevo. Esta llamada es la respuesta
// completa al Requisito 7: agregar "días por rendimiento" es un INSERT.
func (s *Servicio) CrearTipo(ctx context.Context, t domain.TipoDeVacacion) (domain.TipoDeVacacion, error) {
	t.EmpresaID = s.EmpresaID
	if t.VigenteDesde.IsZero() {
		t.VigenteDesde = s.Hoy()
	}
	if err := validarTipo(t); err != nil {
		return domain.TipoDeVacacion{}, err
	}

	id, err := store.CrearTipo(ctx, s.Pool, t)
	if err != nil {
		return domain.TipoDeVacacion{}, err
	}
	t.ID = id
	return t, nil
}

// ActualizarTipo modifica un tipo existente.
func (s *Servicio) ActualizarTipo(ctx context.Context, t domain.TipoDeVacacion) error {
	t.EmpresaID = s.EmpresaID
	if err := validarTipo(t); err != nil {
		return err
	}
	return store.ActualizarTipo(ctx, s.Pool, t)
}

// ListarTipos devuelve la configuración de tipos de la empresa.
func (s *Servicio) ListarTipos(ctx context.Context) ([]domain.TipoDeVacacion, error) {
	return store.ListarTipos(ctx, s.Pool, s.EmpresaID)
}

// validarTipo acota la libertad de configuración. Sin esto, RRHH puede crear un
// tipo que devenga por año calendario sin días base, o uno que vence en n
// períodos sin decir cuántos.
func validarTipo(t domain.TipoDeVacacion) error {
	if t.Codigo == "" {
		return errors.New("el código es obligatorio")
	}
	if t.Nombre == "" {
		return errors.New("el nombre es obligatorio")
	}

	switch t.PoliticaDevengo {
	case domain.DevengoAniversarioLegal, domain.DevengoAnioCalendario:
		if !t.Parametros.DiasBaseDecimal().IsPositive() {
			return errors.New("un tipo con devengo automático necesita días base positivos")
		}
	case domain.DevengoManual:
		// No necesita días base: los define RRHH en cada otorgamiento.
	default:
		return errors.New("política de devengo desconocida")
	}

	switch t.PoliticaVencimiento {
	case domain.VencimientoNPeriodos:
		if t.Parametros.NPeriodos <= 0 {
			return errors.New("la política n_periodos necesita un número de períodos positivo")
		}
	case domain.VencimientoDiasFijos:
		if t.Parametros.DiasFijos <= 0 {
			return errors.New("la política dias_fijos necesita un número de días positivo")
		}
	case domain.VencimientoNoVence, domain.VencimientoFinDeAnio:
		// Sin parámetros.
	default:
		return errors.New("política de vencimiento desconocida")
	}

	if t.Parametros.ProgresivoActivo {
		if t.PoliticaDevengo != domain.DevengoAniversarioLegal {
			return errors.New("el progresivo solo aplica a la política aniversario_legal")
		}
		if t.Parametros.ProgresivoCadenciaAnios <= 0 {
			return errors.New("el progresivo necesita una cadencia en años positiva")
		}
	}

	return nil
}
```

- [ ] **Step 2: Verificar que compila**

Run: `go build ./...`
Expected: sin salida.

- [ ] **Step 3: Commit**

```bash
git add internal/app/rrhh.go
git commit -m "feat(app): otorgamiento manual y ABM de tipos con validación"
```

---

## Task 18: Capa HTTP

Una decisión que atraviesa el API: los decimales viajan en JSON como **string**, no como número. Es el comportamiento por defecto de `shopspring/decimal` y se conserva a propósito: si viajaran como número JSON, el frontend los parsearía a `float64` de JavaScript y el requisito no funcional de aritmética decimal se rompería en el último tramo. El frontend los muestra, no los calcula.

**Files:**
- Create: `internal/http/router.go`, `internal/http/actor.go`, `internal/http/handlers_colaboradores.go`, `internal/http/handlers_solicitudes.go`, `internal/http/handlers_rrhh.go`

- [ ] **Step 1: Crear `internal/http/actor.go`**

```go
package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

type claveContexto string

const claveActor claveContexto = "actor"

// conActor resuelve quién está haciendo la llamada a partir del header
// X-Actor-Id, que el frontend llena desde el selector de usuario.
//
// Esto NO es autenticación: es la demo de un modelo de permisos. En producción
// el actor saldría de un token firmado. Lo que sí es real es el uso que se le
// da: el rol del actor decide qué datos se devuelven.
func (s *Servidor) conActor(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crudo := r.Header.Get("X-Actor-Id")
		if crudo == "" {
			responderError(w, http.StatusUnauthorized, "falta el header X-Actor-Id")
			return
		}
		id, err := uuid.Parse(crudo)
		if err != nil {
			responderError(w, http.StatusBadRequest, "X-Actor-Id no es un uuid válido")
			return
		}

		actor, err := store.ColaboradorPorIDSinEmpresa(r.Context(), s.pool, id)
		if err != nil {
			responderError(w, http.StatusUnauthorized, "actor desconocido")
			return
		}

		ctx := context.WithValue(r.Context(), claveActor, actor)
		siguiente(w, r.WithContext(ctx))
	}
}

// actorDe recupera el actor que el middleware dejó en el contexto.
func actorDe(ctx context.Context) domain.Colaborador {
	actor, _ := ctx.Value(claveActor).(domain.Colaborador)
	return actor
}

// soloRRHH corta la llamada si el actor no tiene rol de RRHH.
func soloRRHH(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !actorDe(r.Context()).EsRRHH() {
			responderError(w, http.StatusForbidden, "esta acción requiere rol de RRHH")
			return
		}
		siguiente(w, r)
	}
}
```

- [ ] **Step 2: Crear `internal/http/router.go`**

```go
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BeSaavedram/vacation-calculator/internal/app"
	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// Servidor expone los casos de uso por HTTP.
type Servidor struct {
	svc  *app.Servicio
	pool *pgxpool.Pool
}

// NuevoServidor arma el router con todas las rutas.
func NuevoServidor(svc *app.Servicio, pool *pgxpool.Pool) http.Handler {
	s := &Servidor{svc: svc, pool: pool}
	mux := http.NewServeMux()

	// Sin actor: alimenta el selector de usuario del frontend.
	mux.HandleFunc("GET /api/usuarios", s.listarUsuarios)
	mux.HandleFunc("GET /api/salud", func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
	})

	// Colaboradores y saldo
	mux.HandleFunc("GET /api/colaboradores", s.conActor(soloRRHH(s.listarColaboradores)))
	mux.HandleFunc("GET /api/colaboradores/{id}/saldo", s.conActor(s.verSaldo))
	mux.HandleFunc("GET /api/colaboradores/{id}/movimientos", s.conActor(s.verHistorial))
	mux.HandleFunc("GET /api/colaboradores/{id}/finiquito", s.conActor(soloRRHH(s.verFiniquito)))

	// Solicitudes
	mux.HandleFunc("GET /api/solicitudes", s.conActor(s.listarSolicitudes))
	mux.HandleFunc("POST /api/solicitudes", s.conActor(s.crearSolicitud))
	mux.HandleFunc("GET /api/solicitudes/preview", s.conActor(s.previewSolicitud))
	mux.HandleFunc("POST /api/solicitudes/{id}/aprobar", s.conActor(soloRRHH(s.aprobarSolicitud)))
	mux.HandleFunc("POST /api/solicitudes/{id}/rechazar", s.conActor(soloRRHH(s.rechazarSolicitud)))

	// Configuración y carga manual
	mux.HandleFunc("GET /api/tipos-vacacion", s.conActor(s.listarTipos))
	mux.HandleFunc("POST /api/tipos-vacacion", s.conActor(soloRRHH(s.crearTipo)))
	mux.HandleFunc("PUT /api/tipos-vacacion/{id}", s.conActor(soloRRHH(s.actualizarTipo)))
	mux.HandleFunc("POST /api/otorgamientos", s.conActor(soloRRHH(s.otorgarManual)))

	// Jobs. Sin interfaz: se explican y se corren con curl.
	mux.HandleFunc("POST /api/jobs/devengo", s.conActor(soloRRHH(s.correrDevengo)))
	mux.HandleFunc("POST /api/jobs/vencimiento", s.conActor(soloRRHH(s.correrVencimiento)))

	return conCORS(mux)
}

// conCORS permite que el frontend de desarrollo en :3000 llame al API en :8080.
func conCORS(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Actor-Id")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		siguiente.ServeHTTP(w, r)
	})
}

func responderJSON(w http.ResponseWriter, codigo int, cuerpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	_ = json.NewEncoder(w).Encode(cuerpo)
}

func responderError(w http.ResponseWriter, codigo int, mensaje string) {
	responderJSON(w, codigo, map[string]string{"error": mensaje})
}

// leerFecha lee un parámetro de query con formato YYYY-MM-DD. Si viene vacío,
// devuelve porDefecto.
func leerFecha(r *http.Request, nombre string, porDefecto time.Time) (time.Time, error) {
	crudo := r.URL.Query().Get(nombre)
	if crudo == "" {
		return porDefecto, nil
	}
	f, err := time.Parse("2006-01-02", crudo)
	if err != nil {
		return time.Time{}, err
	}
	return domain.SoloFecha(f), nil
}
```

- [ ] **Step 3: Crear `internal/http/handlers_colaboradores.go`**

```go
package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// Usuario es una entrada del selector del frontend.
type Usuario struct {
	ID     uuid.UUID `json:"id"`
	Nombre string    `json:"nombre"`
	Rol    string    `json:"rol"`
	Email  string    `json:"email"`
}

// listarUsuarios alimenta el selector "Ver como…". No requiere actor: es el
// punto de entrada de la demo.
func (s *Servidor) listarUsuarios(w http.ResponseWriter, r *http.Request) {
	colaboradores, err := store.ListarColaboradores(r.Context(), s.pool, s.svc.EmpresaID)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}

	usuarios := make([]Usuario, 0, len(colaboradores))
	for _, c := range colaboradores {
		usuarios = append(usuarios, Usuario{ID: c.ID, Nombre: c.Nombre, Rol: string(c.Rol), Email: c.Email})
	}
	responderJSON(w, http.StatusOK, usuarios)
}

// ColaboradorConSaldo es una fila de la tabla de RRHH.
type ColaboradorConSaldo struct {
	ID           uuid.UUID `json:"id"`
	Nombre       string    `json:"nombre"`
	Email        string    `json:"email"`
	Rol          string    `json:"rol"`
	FechaIngreso time.Time `json:"fecha_ingreso"`
	Antiguedad   int       `json:"anios_antiguedad"`
	Disponible   string    `json:"disponible"`
}

func (s *Servidor) listarColaboradores(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hoy := s.svc.Hoy()

	colaboradores, err := store.ListarColaboradores(ctx, s.pool, s.svc.EmpresaID)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}

	filas := make([]ColaboradorConSaldo, 0, len(colaboradores))
	for _, c := range colaboradores {
		saldo, err := s.svc.Saldo(ctx, c.ID, true)
		if err != nil {
			responderError(w, http.StatusInternalServerError, err.Error())
			return
		}
		filas = append(filas, ColaboradorConSaldo{
			ID:           c.ID,
			Nombre:       c.Nombre,
			Email:        c.Email,
			Rol:          string(c.Rol),
			FechaIngreso: c.FechaIngreso,
			Antiguedad:   c.AniosConEmpleadorAl(hoy),
			Disponible:   saldo.Total.String(),
		})
	}
	responderJSON(w, http.StatusOK, filas)
}

// verSaldo devuelve el saldo proyectado. Un colaborador solo puede ver el suyo;
// RRHH puede ver el de cualquiera, y además recibe el devengado no otorgado.
func (s *Servidor) verSaldo(w http.ResponseWriter, r *http.Request) {
	actor := actorDe(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if !actor.EsRRHH() && actor.ID != id {
		responderError(w, http.StatusForbidden, "solo puedes consultar tu propio saldo")
		return
	}

	saldo, err := s.svc.Saldo(r.Context(), id, actor.EsRRHH())
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, saldo)
}

// verHistorial devuelve el ledger. Con ?hasta=YYYY-MM-DD reconstruye el saldo a
// una fecha pasada: es la demostración directa del Requisito 5.
func (s *Servidor) verHistorial(w http.ResponseWriter, r *http.Request) {
	actor := actorDe(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if !actor.EsRRHH() && actor.ID != id {
		responderError(w, http.StatusForbidden, "solo puedes consultar tu propio historial")
		return
	}

	var hasta *time.Time
	if crudo := r.URL.Query().Get("hasta"); crudo != "" {
		f, err := time.Parse("2006-01-02", crudo)
		if err != nil {
			responderError(w, http.StatusBadRequest, "hasta debe tener formato YYYY-MM-DD")
			return
		}
		hasta = &f
	}

	movimientos, err := s.svc.Historial(r.Context(), id, hasta)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if movimientos == nil {
		movimientos = []store.MovimientoConContexto{}
	}
	responderJSON(w, http.StatusOK, movimientos)
}

func (s *Servidor) verFiniquito(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}
	fecha, err := leerFecha(r, "fecha", s.svc.Hoy())
	if err != nil {
		responderError(w, http.StatusBadRequest, "fecha debe tener formato YYYY-MM-DD")
		return
	}

	finiquito, err := s.svc.Finiquito(r.Context(), id, fecha)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, finiquito)
}
```

- [ ] **Step 4: Crear `internal/http/handlers_solicitudes.go`**

```go
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/app"
	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

func (s *Servidor) listarSolicitudes(w http.ResponseWriter, r *http.Request) {
	solicitudes, err := s.svc.ListarSolicitudes(r.Context(), actorDe(r.Context()))
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if solicitudes == nil {
		solicitudes = []store.Solicitud{}
	}
	responderJSON(w, http.StatusOK, solicitudes)
}

// previewSolicitud cuenta los días hábiles de un rango antes de confirmar, para
// que el colaborador vea el descuento real y no el número de días corridos.
func (s *Servidor) previewSolicitud(w http.ResponseWriter, r *http.Request) {
	desde, err := leerFecha(r, "desde", time.Time{})
	if err != nil || desde.IsZero() {
		responderError(w, http.StatusBadRequest, "desde es obligatorio con formato YYYY-MM-DD")
		return
	}
	hasta, err := leerFecha(r, "hasta", time.Time{})
	if err != nil || hasta.IsZero() {
		responderError(w, http.StatusBadRequest, "hasta es obligatorio con formato YYYY-MM-DD")
		return
	}
	if hasta.Before(desde) {
		responderError(w, http.StatusBadRequest, "hasta no puede ser anterior a desde")
		return
	}

	preview, err := s.svc.PreviewDiasHabiles(r.Context(), desde, hasta)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, preview)
}

type cuerpoCrearSolicitud struct {
	TipoID string `json:"tipo_id"`
	Desde  string `json:"desde"`
	Hasta  string `json:"hasta"`
}

func (s *Servidor) crearSolicitud(w http.ResponseWriter, r *http.Request) {
	actor := actorDe(r.Context())

	var cuerpo cuerpoCrearSolicitud
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	tipoID, err := uuid.Parse(cuerpo.TipoID)
	if err != nil {
		responderError(w, http.StatusBadRequest, "tipo_id inválido")
		return
	}
	desde, err := time.Parse("2006-01-02", cuerpo.Desde)
	if err != nil {
		responderError(w, http.StatusBadRequest, "desde debe tener formato YYYY-MM-DD")
		return
	}
	hasta, err := time.Parse("2006-01-02", cuerpo.Hasta)
	if err != nil {
		responderError(w, http.StatusBadRequest, "hasta debe tener formato YYYY-MM-DD")
		return
	}

	solicitud, err := s.svc.CrearSolicitud(r.Context(), actor.ID, tipoID, desde, hasta)
	switch {
	case errors.Is(err, domain.ErrSaldoInsuficiente), errors.Is(err, app.ErrRangoInvalido):
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case err != nil:
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusCreated, solicitud)
}

func (s *Servidor) aprobarSolicitud(w http.ResponseWriter, r *http.Request) {
	s.decidirSolicitud(w, r, true)
}

func (s *Servidor) rechazarSolicitud(w http.ResponseWriter, r *http.Request) {
	s.decidirSolicitud(w, r, false)
}

func (s *Servidor) decidirSolicitud(w http.ResponseWriter, r *http.Request, aprobar bool) {
	actor := actorDe(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}

	if aprobar {
		err = s.svc.AprobarSolicitud(r.Context(), id, actor.ID)
	} else {
		err = s.svc.RechazarSolicitud(r.Context(), id, actor.ID)
	}

	switch {
	case errors.Is(err, app.ErrYaDecidida):
		responderError(w, http.StatusConflict, err.Error())
		return
	case errors.Is(err, domain.ErrSaldoInsuficiente):
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case err != nil:
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}

	solicitud, err := store.SolicitudPorID(r.Context(), s.pool, s.svc.EmpresaID, id)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, solicitud)
}
```

- [ ] **Step 5: Crear `internal/http/handlers_rrhh.go`**

```go
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

func (s *Servidor) listarTipos(w http.ResponseWriter, r *http.Request) {
	tipos, err := s.svc.ListarTipos(r.Context())
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tipos == nil {
		tipos = []domain.TipoDeVacacion{}
	}
	responderJSON(w, http.StatusOK, tipos)
}

// cuerpoTipo es la forma en que el frontend describe un tipo de vacación.
type cuerpoTipo struct {
	Codigo              string            `json:"codigo"`
	Nombre              string            `json:"nombre"`
	PoliticaDevengo     string            `json:"politica_devengo"`
	PoliticaVencimiento string            `json:"politica_vencimiento"`
	Parametros          domain.Parametros `json:"parametros"`
	PrioridadConsumo    int               `json:"prioridad_consumo"`
	UnidadHabil         bool              `json:"unidad_habil"`
	PagableEnFiniquito  bool              `json:"pagable_en_finiquito"`
}

func (c cuerpoTipo) aDominio() domain.TipoDeVacacion {
	return domain.TipoDeVacacion{
		Codigo:              c.Codigo,
		Nombre:              c.Nombre,
		PoliticaDevengo:     domain.CodigoDevengo(c.PoliticaDevengo),
		PoliticaVencimiento: domain.CodigoVencimiento(c.PoliticaVencimiento),
		Parametros:          c.Parametros,
		PrioridadConsumo:    c.PrioridadConsumo,
		UnidadHabil:         c.UnidadHabil,
		PagableEnFiniquito:  c.PagableEnFiniquito,
	}
}

// crearTipo es el Requisito 7 completo: crear "días por rendimiento" es esta
// llamada, sin desarrollo ni despliegue.
func (s *Servidor) crearTipo(w http.ResponseWriter, r *http.Request) {
	var cuerpo cuerpoTipo
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	tipo, err := s.svc.CrearTipo(r.Context(), cuerpo.aDominio())
	if err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusCreated, tipo)
}

func (s *Servidor) actualizarTipo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}

	var cuerpo cuerpoTipo
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	tipo := cuerpo.aDominio()
	tipo.ID = id
	if err := s.svc.ActualizarTipo(r.Context(), tipo); err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, tipo)
}

type cuerpoOtorgamiento struct {
	ColaboradorID string `json:"colaborador_id"`
	TipoID        string `json:"tipo_id"`
	Dias          string `json:"dias"`
	Motivo        string `json:"motivo"`
}

// otorgarManual carga un saldo especial. El motivo es obligatorio y queda
// escrito en el ledger junto al actor que lo cargó.
func (s *Servidor) otorgarManual(w http.ResponseWriter, r *http.Request) {
	actor := actorDe(r.Context())

	var cuerpo cuerpoOtorgamiento
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	colaboradorID, err := uuid.Parse(cuerpo.ColaboradorID)
	if err != nil {
		responderError(w, http.StatusBadRequest, "colaborador_id inválido")
		return
	}
	tipoID, err := uuid.Parse(cuerpo.TipoID)
	if err != nil {
		responderError(w, http.StatusBadRequest, "tipo_id inválido")
		return
	}
	dias, err := decimal.NewFromString(cuerpo.Dias)
	if err != nil {
		responderError(w, http.StatusBadRequest, "dias debe ser un número decimal, por ejemplo \"2.5\"")
		return
	}

	if err := s.svc.OtorgarManual(r.Context(), colaboradorID, tipoID, dias, cuerpo.Motivo, actor.ID); err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusCreated, map[string]string{"estado": "otorgado"})
}

func (s *Servidor) correrDevengo(w http.ResponseWriter, r *http.Request) {
	s.correrJob(w, r, s.svc.CorrerDevengo)
}

func (s *Servidor) correrVencimiento(w http.ResponseWriter, r *http.Request) {
	s.correrJob(w, r, s.svc.CorrerVencimiento)
}

type funcionJob func(ctx context.Context, fecha time.Time, actorID uuid.UUID) (app.ResultadoJob, error)

func (s *Servidor) correrJob(w http.ResponseWriter, r *http.Request, job funcionJob) {
	actor := actorDe(r.Context())

	fecha, err := leerFecha(r, "fecha", s.svc.Hoy())
	if err != nil {
		responderError(w, http.StatusBadRequest, "fecha debe tener formato YYYY-MM-DD")
		return
	}

	resultado, err := job(r.Context(), fecha, actor.ID)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, resultado)
}
```

Agregar `"context"` y el import de `app` a este archivo:

```go
import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/app"
	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)
```

- [ ] **Step 6: Verificar que compila**

Run: `go build ./...`
Expected: sin salida.

- [ ] **Step 7: Commit**

```bash
git add internal/http
git commit -m "feat(http): router, middleware de actor y handlers"
```

---

## Task 19: Arranque del API

**Files:**
- Create: `cmd/api/main.go`

- [ ] **Step 1: Crear `cmd/api/main.go`**

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BeSaavedram/vacation-calculator/internal/app"
	apihttp "github.com/BeSaavedram/vacation-calculator/internal/http"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("falta DATABASE_URL")
	}
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8080"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("conectando a la base: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("la base no responde: %v", err)
	}

	empresaID, err := empresaUnica(ctx, pool)
	if err != nil {
		log.Fatalf("resolviendo la empresa: %v (¿corriste make seed?)", err)
	}

	servicio := app.NuevoServicio(pool, empresaID)
	servidor := &http.Server{
		Addr:              ":" + puerto,
		Handler:           apihttp.NuevoServidor(servicio, pool),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API escuchando en http://localhost:%s", puerto)
	if err := servidor.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// empresaUnica resuelve el id de la empresa. Este MVP opera con una sola, pero
// el id viaja explícito en cada consulta: la multiempresa está en la estructura
// aunque no esté en la interfaz.
func empresaUnica(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM empresa ORDER BY razon_social LIMIT 1`).Scan(&id)
	return id, err
}
```

- [ ] **Step 2: Verificar que compila**

Run: `go build ./...`
Expected: sin salida.

- [ ] **Step 3: Commit**

```bash
git add cmd/api
git commit -m "feat(api): arranque del servidor"
```

---

## Task 20: Semilla

La semilla no inserta saldos: **corre el motor** sobre la historia completa de cada colaborador. Esa diferencia es lo que hace que el ledger de Carlos sirva para explicar el sistema en vez de solo ilustrarlo.

**Files:**
- Create: `cmd/seed/main.go`, `internal/store/feriados.go`

- [ ] **Step 1: Crear `internal/store/feriados.go`**

Feriados legales de Chile. Se necesitan desde 2017 porque el conteo histórico de Carlos es real.

```go
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
```

- [ ] **Step 2: Crear `cmd/seed/main.go`**

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BeSaavedram/vacation-calculator/internal/app"
	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// hoy es la fecha de referencia de la demo. Se fija explícitamente para que la
// historia sembrada sea reproducible.
var hoy = domain.SoloFecha(time.Now().UTC())

func main() {
	dsn := os.Getenv("DATABASE_URL_OWNER")
	if dsn == "" {
		log.Fatal("falta DATABASE_URL_OWNER")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("conectando: %v", err)
	}
	defer pool.Close()

	if err := limpiar(ctx, pool); err != nil {
		log.Fatalf("limpiando: %v", err)
	}

	empresaID, err := crearEmpresa(ctx, pool)
	if err != nil {
		log.Fatalf("creando empresa: %v", err)
	}

	if err := store.InsertarFeriados(ctx, pool, store.FeriadosChile(2015, hoy.Year()+2)); err != nil {
		log.Fatalf("cargando calendario: %v", err)
	}

	marta, ana, carlos, err := crearColaboradores(ctx, pool, empresaID)
	if err != nil {
		log.Fatalf("creando colaboradores: %v", err)
	}

	if err := crearTipos(ctx, pool, empresaID); err != nil {
		log.Fatalf("creando tipos: %v", err)
	}

	servicio := app.NuevoServicio(pool, empresaID)

	// Aquí está el punto: no insertamos saldos. Recorremos la historia día por
	// día corriendo el mismo motor que corre en producción. El ledger que queda
	// es el que el sistema habría producido si hubiera estado funcionando desde
	// el primer día.
	if err := recorrerHistoria(ctx, servicio, marta); err != nil {
		log.Fatalf("recorriendo la historia: %v", err)
	}

	if err := sembrarSolicitudes(ctx, servicio, ana, carlos, marta, empresaID, pool); err != nil {
		log.Fatalf("sembrando solicitudes: %v", err)
	}

	log.Println("semilla lista")
	log.Printf("  RRHH        %s", marta)
	log.Printf("  Ana         %s", ana)
	log.Printf("  Carlos      %s", carlos)
}

func limpiar(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		TRUNCATE movimiento, solicitud_de_vacaciones, otorgamiento,
		         tipo_de_vacacion, colaborador, empresa, calendario_laboral
		RESTART IDENTITY CASCADE`)
	return err
}

func crearEmpresa(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO empresa (razon_social) VALUES ($1) RETURNING id`,
		"Compañía Demo S.A.").Scan(&id)
	return id, err
}

func crearColaboradores(ctx context.Context, pool *pgxpool.Pool, empresaID uuid.UUID) (marta, ana, carlos uuid.UUID, err error) {
	marta, err = store.CrearColaborador(ctx, pool, domain.Colaborador{
		EmpresaID:    empresaID,
		Nombre:       "Marta Silva",
		Email:        "marta.silva@demo.cl",
		Rol:          domain.RolRRHH,
		FechaIngreso: domain.Fecha(2022, time.January, 10),
		Jornada:      "completa",
	})
	if err != nil {
		return
	}

	// Caso simple: dos aniversarios cumplidos, sin progresivas.
	ana, err = store.CrearColaborador(ctx, pool, domain.Colaborador{
		EmpresaID:    empresaID,
		Nombre:       "Ana Fuentes",
		Email:        "ana.fuentes@demo.cl",
		Rol:          domain.RolColaborador,
		FechaIngreso: domain.Fecha(2024, time.March, 1),
		Jornada:      "completa",
	})
	if err != nil {
		return
	}

	// Caso rico: nueve años con el empleador y 24 meses de experiencia previa
	// acreditada. Cruza el umbral de las progresivas en dos escalones visibles,
	// 15 → 17 → 18, y acumula vencimientos por el tope de dos períodos.
	carlos, err = store.CrearColaborador(ctx, pool, domain.Colaborador{
		EmpresaID:              empresaID,
		Nombre:                 "Carlos Rojas",
		Email:                  "carlos.rojas@demo.cl",
		Rol:                    domain.RolColaborador,
		FechaIngreso:           domain.Fecha(2017, time.April, 15),
		MesesExperienciaPrevia: 24,
		Jornada:                "completa",
	})
	return
}

func crearTipos(ctx context.Context, pool *pgxpool.Pool, empresaID uuid.UUID) error {
	tipos := []domain.TipoDeVacacion{
		{
			EmpresaID:           empresaID,
			Codigo:              "FERIADO_LEGAL",
			Nombre:              "Feriado legal",
			PoliticaDevengo:     domain.DevengoAniversarioLegal,
			PoliticaVencimiento: domain.VencimientoNPeriodos,
			PrioridadConsumo:    20,
			UnidadHabil:         true,
			PagableEnFiniquito:  true,
			VigenteDesde:        domain.Fecha(2000, time.January, 1),
			Parametros: domain.Parametros{
				DiasBase:                        "15",
				ProgresivoActivo:                true,
				ProgresivoUmbralMeses:           120,
				ProgresivoAntiguedadMinimaMeses: 36,
				ProgresivoCadenciaAnios:         3,
				ProgresivoDiasPorTramo:          "1",
				NPeriodos:                       2,
			},
		},
		{
			EmpresaID:           empresaID,
			Codigo:              "ADMINISTRATIVO",
			Nombre:              "Días administrativos",
			PoliticaDevengo:     domain.DevengoAnioCalendario,
			PoliticaVencimiento: domain.VencimientoFinDeAnio,
			// Prioridad menor que el feriado legal: se gastan primero porque
			// mueren antes.
			PrioridadConsumo:   10,
			UnidadHabil:        true,
			PagableEnFiniquito: false,
			VigenteDesde:       domain.Fecha(2000, time.January, 1),
			Parametros:         domain.Parametros{DiasBase: "6"},
		},
	}

	for _, t := range tipos {
		if _, err := store.CrearTipo(ctx, pool, t); err != nil {
			return err
		}
	}
	// RENDIMIENTO se deja sin crear a propósito: se crea en vivo durante la
	// demo, desde el ABM, para mostrar el Requisito 7.
	return nil
}

// recorrerHistoria corre el motor de devengo y el de vencimiento para cada día
// relevante desde el ingreso más antiguo hasta hoy.
//
// Solo se evalúan los días que pueden producir algo: aniversarios, primeros de
// enero y fechas de vencimiento. Recorrer nueve años día por día también
// funcionaría, pero tardaría más sin cambiar el resultado, porque el motor es
// idempotente y devuelve "no hubo devengo" en los demás días.
func recorrerHistoria(ctx context.Context, svc *app.Servicio, actorID uuid.UUID) error {
	fechas, err := fechasRelevantes(ctx, svc)
	if err != nil {
		return err
	}

	for _, f := range fechas {
		svc.Hoy = func() time.Time { return f }

		if _, err := svc.CorrerDevengo(ctx, f, actorID); err != nil {
			return err
		}
		if _, err := svc.CorrerVencimiento(ctx, f, actorID); err != nil {
			return err
		}
	}

	svc.Hoy = func() time.Time { return hoy }
	return nil
}

// fechasRelevantes reúne, en orden, todas las fechas en que el motor puede
// producir un movimiento.
func fechasRelevantes(ctx context.Context, svc *app.Servicio) ([]time.Time, error) {
	colaboradores, err := store.ListarColaboradores(ctx, svc.Pool, svc.EmpresaID)
	if err != nil {
		return nil, err
	}
	tipos, err := store.ListarTipos(ctx, svc.Pool, svc.EmpresaID)
	if err != nil {
		return nil, err
	}

	conjunto := make(map[string]time.Time)
	agregar := func(f time.Time) {
		if !f.After(hoy) {
			conjunto[f.Format("2006-01-02")] = f
		}
	}

	for _, c := range colaboradores {
		for anio := c.FechaIngreso.Year(); anio <= hoy.Year(); anio++ {
			aniversario := domain.Fecha(anio, c.FechaIngreso.Month(), c.FechaIngreso.Day())
			agregar(aniversario)

			// Las fechas de vencimiento derivadas de ese aniversario.
			for _, t := range tipos {
				if v := domain.CalcularVencimiento(t, aniversario); v != nil {
					agregar(*v)
				}
			}
		}
		for anio := c.FechaIngreso.Year(); anio <= hoy.Year(); anio++ {
			agregar(domain.Fecha(anio, time.January, 1))
		}
	}

	fechas := make([]time.Time, 0, len(conjunto))
	for _, f := range conjunto {
		fechas = append(fechas, f)
	}
	sort.Slice(fechas, func(i, j int) bool { return fechas[i].Before(fechas[j]) })
	return fechas, nil
}

// sembrarSolicitudes crea y aprueba unas vacaciones históricas, para que el
// ledger muestre consumos repartidos FIFO y los vencimientos sean por
// remanentes parciales y no por bolsas intactas.
func sembrarSolicitudes(
	ctx context.Context, svc *app.Servicio,
	ana, carlos, marta, empresaID uuid.UUID, pool *pgxpool.Pool,
) error {
	legal, err := store.TipoPorCodigo(ctx, pool, empresaID, "FERIADO_LEGAL")
	if err != nil {
		return err
	}

	vacaciones := []struct {
		colaborador  uuid.UUID
		desde, hasta time.Time
	}{
		{carlos, domain.Fecha(hoy.Year()-1, time.February, 3), domain.Fecha(hoy.Year()-1, time.February, 14)},
		{carlos, domain.Fecha(hoy.Year(), time.January, 6), domain.Fecha(hoy.Year(), time.January, 10)},
		{ana, domain.Fecha(hoy.Year(), time.February, 10), domain.Fecha(hoy.Year(), time.February, 14)},
	}

	for _, v := range vacaciones {
		solicitud, err := svc.CrearSolicitud(ctx, v.colaborador, legal.ID, v.desde, v.hasta)
		if err != nil {
			return err
		}
		if err := svc.AprobarSolicitud(ctx, solicitud.ID, marta); err != nil {
			return err
		}
	}

	// Una solicitud pendiente, para que la bandeja de RRHH no esté vacía al
	// abrir la demo.
	_, err = svc.CrearSolicitud(ctx, ana, legal.ID,
		hoy.AddDate(0, 1, 0), hoy.AddDate(0, 1, 4))
	return err
}
```

Agregar `"sort"` a los imports de este archivo.

**Cuidado con el orden de los argumentos:** `crearColaboradores` devuelve `(marta, ana, carlos)` y `sembrarSolicitudes` los recibe como `(ana, carlos, marta, ...)`. La llamada en `main` ya los pasa en el orden correcto: `sembrarSolicitudes(ctx, servicio, ana, carlos, marta, empresaID, pool)`.

- [ ] **Step 3: Correr la semilla**

```bash
make demo-reset
```

Expected: termina con `semilla lista` y los tres uuid.

- [ ] **Step 4: Verificar que el ledger de Carlos cuenta la historia**

```bash
docker compose exec -T postgres psql -U vacaciones -d vacaciones -c "SELECT m.fecha_efectiva, m.clase, m.cantidad, t.codigo FROM movimiento m JOIN otorgamiento o ON o.id = m.otorgamiento_id JOIN colaborador c ON c.id = o.colaborador_id JOIN tipo_de_vacacion t ON t.id = o.tipo_id WHERE c.nombre = 'Carlos Rojas' ORDER BY m.fecha_efectiva;"
```

Expected: `ACCRUAL` de `15.00` en los aniversarios de 2018 a 2024, **`17.00` en 2025-04-15** y **`18.00` en 2026-04-15**, más `EXPIRATION` negativos y `CONSUMPTION` negativos.

Si los montos de 2025 y 2026 no son 17 y 18, el motor de progresivas o la experiencia previa de Carlos están mal: revisar antes de seguir. Ese salto es el centro de la demo.

- [ ] **Step 5: Commit**

```bash
git add cmd/seed internal/store/feriados.go
git commit -m "feat(seed): historia generada corriendo el motor, no insertando saldos"
```

---

## Task 21: Verificación del API de punta a punta

Antes de tocar el frontend, comprobar que el API responde lo que la demo necesita.

**Files:** ninguno. Es una verificación.

- [ ] **Step 1: Levantar el API**

```bash
make api
```

Expected: `API escuchando en http://localhost:8080`

- [ ] **Step 2: Obtener los ids de los usuarios**

```bash
curl -s http://localhost:8080/api/usuarios | python3 -m json.tool
```

Expected: tres usuarios, uno con `"rol": "RRHH"`.

- [ ] **Step 3: Verificar que el colaborador NO ve el devengado no otorgado**

Reemplazar `CARLOS_ID` por el uuid de Carlos en ambos comandos.

```bash
curl -s -H "X-Actor-Id: CARLOS_ID" http://localhost:8080/api/colaboradores/CARLOS_ID/saldo | python3 -m json.tool
```

Expected: el JSON **no** contiene `devengado_no_otorgado` ni `proporcional`.

- [ ] **Step 4: Verificar que RRHH sí los ve**

Reemplazar `MARTA_ID` y `CARLOS_ID`.

```bash
curl -s -H "X-Actor-Id: MARTA_ID" http://localhost:8080/api/colaboradores/CARLOS_ID/saldo | python3 -m json.tool
```

Expected: el JSON **sí** contiene `devengado_no_otorgado` y `proporcional` con su desglose de meses y días.

- [ ] **Step 5: Verificar la idempotencia del devengo**

Correr dos veces el job para el aniversario de Carlos:

```bash
curl -s -X POST -H "X-Actor-Id: MARTA_ID" "http://localhost:8080/api/jobs/devengo?fecha=2026-04-15" | python3 -m json.tool
curl -s -X POST -H "X-Actor-Id: MARTA_ID" "http://localhost:8080/api/jobs/devengo?fecha=2026-04-15" | python3 -m json.tool
```

Expected: ambas corridas devuelven `"creados": 0` y `"ya_existian": 1`, porque la semilla ya procesó esa fecha. Lo importante es que el número de movimientos en la base no cambie.

- [ ] **Step 6: Verificar el finiquito**

```bash
curl -s -H "X-Actor-Id: MARTA_ID" "http://localhost:8080/api/colaboradores/CARLOS_ID/finiquito" | python3 -m json.tool
```

Expected: un objeto con `proporcional` (meses, días y total) y `total`.

- [ ] **Step 7: Commit**

No hay archivos que commitear. Si alguna verificación falló, corregir el código correspondiente y commitear la corrección antes de seguir.

---

## Task 22: Frontend — esqueleto, cliente del API y selector de usuario

Todas las páginas son componentes de cliente. La razón es concreta: cada llamada necesita el header `X-Actor-Id`, que sale del selector de usuario guardado en el navegador. Un componente de servidor no tiene acceso a eso sin montar una capa de sesión que este MVP no necesita.

Los decimales llegan del API como **string** y se muestran como string. El frontend nunca los convierte a `number`: no hace aritmética con ellos, y convertirlos rompería el requisito de precisión en el último tramo.

**Files:**
- Create: `web/` (scaffold), `web/lib/tipos.ts`, `web/lib/api.ts`, `web/lib/actor.tsx`, `web/components/SelectorUsuario.tsx`, `web/components/ui.tsx`, `web/app/layout.tsx`, `web/app/page.tsx`

- [ ] **Step 1: Generar el proyecto Next.js**

```bash
npx create-next-app@latest web --ts --tailwind --app --no-src-dir --import-alias "@/*" --eslint --no-turbopack --use-npm
```

- [ ] **Step 2: Configurar la URL del API en `web/.env.local`**

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

- [ ] **Step 3: Crear `web/lib/tipos.ts`**

```typescript
// Los campos decimales son string a propósito: el API los serializa así para
// que nunca pasen por el float64 de JavaScript. Se muestran, no se calculan.

export type Rol = "COLABORADOR" | "RRHH";

export interface Usuario {
  id: string;
  nombre: string;
  rol: Rol;
  email: string;
}

export interface BolsaPorVencer {
  OtorgamientoID: string;
  Dias: string;
  VenceEl: string;
}

export interface SaldoDeTipo {
  tipo_id: string;
  tipo_codigo: string;
  tipo_nombre: string;
  disponible: string;
  pendiente: string;
  solicitable: string;
  por_vencer: BolsaPorVencer[] | null;
}

export interface Proporcional {
  PeriodoDesde: string;
  Hasta: string;
  MesesCompletos: number;
  DiasRestantes: number;
  Dias: string;
}

export interface Saldo {
  colaborador_id: string;
  colaborador_nombre: string;
  fecha_ingreso: string;
  al_dia: string;
  total_disponible: string;
  por_tipo: SaldoDeTipo[] | null;
  devengado_no_otorgado?: string;
  proporcional?: Proporcional;
}

export interface Movimiento {
  ID: string;
  OtorgamientoID: string;
  SolicitudID: string | null;
  Cantidad: string;
  Clase: string;
  FechaEfectiva: string;
  FechaRegistro: string;
  Motivo: string;
  ClaveIdempotencia: string;
  TipoCodigo: string;
  TipoNombre: string;
  ActorNombre: string;
  VenceEl: string | null;
}

export interface Solicitud {
  ID: string;
  ColaboradorID: string;
  ColaboradorNom: string;
  TipoID: string;
  TipoCodigo: string;
  Desde: string;
  Hasta: string;
  DiasHabiles: string;
  Estado: "PENDIENTE" | "APROBADA" | "RECHAZADA" | "CANCELADA";
  CreadaEl: string;
}

export interface ColaboradorConSaldo {
  id: string;
  nombre: string;
  email: string;
  rol: Rol;
  fecha_ingreso: string;
  anios_antiguedad: number;
  disponible: string;
}

export interface Parametros {
  dias_base?: string;
  progresivo_activo?: boolean;
  progresivo_umbral_meses?: number;
  progresivo_antiguedad_minima_meses?: number;
  progresivo_cadencia_anios?: number;
  progresivo_dias_por_tramo?: string;
  n_periodos?: number;
  dias_fijos?: number;
}

export interface TipoDeVacacion {
  ID: string;
  Codigo: string;
  Nombre: string;
  PoliticaDevengo: string;
  PoliticaVencimiento: string;
  Parametros: Parametros;
  PrioridadConsumo: number;
  UnidadHabil: boolean;
  PagableEnFiniquito: boolean;
}

export interface Finiquito {
  Proporcional: Proporcional;
  DisponiblePagable: string;
  Total: string;
}

export interface Preview {
  desde: string;
  hasta: string;
  dias_habiles: string;
  dias_corridos: number;
}
```

- [ ] **Step 4: Crear `web/lib/api.ts`**

```typescript
const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ErrorApi extends Error {}

// pedir hace la llamada agregando el header de actor. Todo el frontend pasa por
// acá: no hay otra forma de hablar con el API.
export async function pedir<T>(
  ruta: string,
  actorId: string | null,
  init?: RequestInit,
): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (actorId) headers["X-Actor-Id"] = actorId;

  const respuesta = await fetch(`${BASE}${ruta}`, { ...init, headers });

  if (!respuesta.ok) {
    let mensaje = `${respuesta.status} ${respuesta.statusText}`;
    try {
      const cuerpo = await respuesta.json();
      if (cuerpo?.error) mensaje = cuerpo.error;
    } catch {
      // La respuesta no era JSON; nos quedamos con el status.
    }
    throw new ErrorApi(mensaje);
  }

  if (respuesta.status === 204) return undefined as T;
  return respuesta.json() as Promise<T>;
}

export function postear<T>(ruta: string, actorId: string | null, cuerpo?: unknown) {
  return pedir<T>(ruta, actorId, {
    method: "POST",
    body: cuerpo === undefined ? undefined : JSON.stringify(cuerpo),
  });
}
```

- [ ] **Step 5: Crear `web/lib/actor.tsx`**

```tsx
"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { pedir } from "./api";
import type { Usuario } from "./tipos";

interface ContextoActor {
  actor: Usuario | null;
  usuarios: Usuario[];
  cambiarActor: (id: string) => void;
  cargando: boolean;
}

const Contexto = createContext<ContextoActor>({
  actor: null,
  usuarios: [],
  cambiarActor: () => {},
  cargando: true,
});

const CLAVE = "vacaciones.actor";

export function ProveedorActor({ children }: { children: React.ReactNode }) {
  const [usuarios, setUsuarios] = useState<Usuario[]>([]);
  const [actor, setActor] = useState<Usuario | null>(null);
  const [cargando, setCargando] = useState(true);

  useEffect(() => {
    pedir<Usuario[]>("/api/usuarios", null)
      .then((lista) => {
        setUsuarios(lista);
        const guardado = localStorage.getItem(CLAVE);
        const elegido = lista.find((u) => u.id === guardado) ?? lista[0] ?? null;
        setActor(elegido);
      })
      .catch(() => setUsuarios([]))
      .finally(() => setCargando(false));
  }, []);

  function cambiarActor(id: string) {
    const elegido = usuarios.find((u) => u.id === id);
    if (!elegido) return;
    localStorage.setItem(CLAVE, id);
    setActor(elegido);
  }

  return (
    <Contexto.Provider value={{ actor, usuarios, cambiarActor, cargando }}>
      {children}
    </Contexto.Provider>
  );
}

export function useActor() {
  return useContext(Contexto);
}
```

- [ ] **Step 6: Crear `web/components/ui.tsx`**

Piezas visuales compartidas, para no repetir clases de Tailwind en cada página.

```tsx
export function Tarjeta({
  titulo,
  children,
}: {
  titulo?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      {titulo && (
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-slate-500">
          {titulo}
        </h2>
      )}
      {children}
    </section>
  );
}

export function Etiqueta({ valor }: { valor: string }) {
  const colores: Record<string, string> = {
    PENDIENTE: "bg-amber-100 text-amber-800",
    APROBADA: "bg-emerald-100 text-emerald-800",
    RECHAZADA: "bg-rose-100 text-rose-800",
    CANCELADA: "bg-slate-100 text-slate-600",
    ACCRUAL: "bg-emerald-100 text-emerald-800",
    CONSUMPTION: "bg-sky-100 text-sky-800",
    EXPIRATION: "bg-rose-100 text-rose-800",
    ADJUSTMENT: "bg-violet-100 text-violet-800",
    REVERSAL: "bg-orange-100 text-orange-800",
    OPENING_BALANCE: "bg-slate-100 text-slate-700",
    SETTLEMENT_PAYOUT: "bg-indigo-100 text-indigo-800",
  };
  return (
    <span
      className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${
        colores[valor] ?? "bg-slate-100 text-slate-700"
      }`}
    >
      {valor}
    </span>
  );
}

export function Aviso({ mensaje, tono = "error" }: { mensaje: string; tono?: "error" | "ok" }) {
  const clases =
    tono === "ok"
      ? "border-emerald-200 bg-emerald-50 text-emerald-800"
      : "border-rose-200 bg-rose-50 text-rose-800";
  return <p className={`rounded border px-3 py-2 text-sm ${clases}`}>{mensaje}</p>;
}

// fecha formatea un ISO del API como YYYY-MM-DD, sin zona horaria.
export function fecha(iso: string): string {
  return iso.slice(0, 10);
}
```

- [ ] **Step 7: Reemplazar `web/app/layout.tsx`**

```tsx
import type { Metadata } from "next";
import "./globals.css";
import { ProveedorActor } from "@/lib/actor";
import { SelectorUsuario } from "@/components/SelectorUsuario";

export const metadata: Metadata = {
  title: "Gestión de Vacaciones",
  description: "Saldos de vacaciones derivados de un ledger inmutable",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="es">
      <body className="min-h-screen bg-slate-50 text-slate-900 antialiased">
        <ProveedorActor>
          <SelectorUsuario />
          <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
        </ProveedorActor>
      </body>
    </html>
  );
}
```

- [ ] **Step 8: Crear `web/components/SelectorUsuario.tsx`**

```tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useActor } from "@/lib/actor";

export function SelectorUsuario() {
  const { actor, usuarios, cambiarActor } = useActor();
  const ruta = usePathname();

  const enlaces = actor?.rol === "RRHH"
    ? [
        { href: "/rrhh", texto: "Colaboradores" },
        { href: "/rrhh/solicitudes", texto: "Solicitudes" },
        { href: "/rrhh/tipos", texto: "Tipos de vacación" },
      ]
    : [{ href: "/mis-vacaciones", texto: "Mis vacaciones" }];

  return (
    <header className="border-b border-slate-200 bg-white">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-6 px-6 py-3">
        <div className="flex items-center gap-6">
          <span className="font-semibold">Gestión de Vacaciones</span>
          <nav className="flex gap-4 text-sm">
            {enlaces.map((e) => (
              <Link
                key={e.href}
                href={e.href}
                className={
                  ruta === e.href
                    ? "font-medium text-slate-900"
                    : "text-slate-500 hover:text-slate-900"
                }
              >
                {e.texto}
              </Link>
            ))}
          </nav>
        </div>

        <label className="flex items-center gap-2 text-sm">
          <span className="text-slate-500">Ver como</span>
          <select
            value={actor?.id ?? ""}
            onChange={(e) => cambiarActor(e.target.value)}
            className="rounded border border-slate-300 bg-white px-2 py-1"
          >
            {usuarios.map((u) => (
              <option key={u.id} value={u.id}>
                {u.nombre} · {u.rol === "RRHH" ? "RRHH" : "Colaborador"}
              </option>
            ))}
          </select>
        </label>
      </div>
    </header>
  );
}
```

- [ ] **Step 9: Reemplazar `web/app/page.tsx`**

La raíz solo redirige según el rol del usuario seleccionado.

```tsx
"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useActor } from "@/lib/actor";

export default function Inicio() {
  const { actor, cargando } = useActor();
  const router = useRouter();

  useEffect(() => {
    if (cargando || !actor) return;
    router.replace(actor.rol === "RRHH" ? "/rrhh" : "/mis-vacaciones");
  }, [actor, cargando, router]);

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (!actor) {
    return (
      <p className="text-slate-500">
        No hay usuarios. ¿Corriste <code>make seed</code> y está el API arriba?
      </p>
    );
  }
  return <p className="text-slate-500">Redirigiendo…</p>;
}
```

- [ ] **Step 10: Verificar que el frontend levanta**

```bash
make web
```

Expected: abrir `http://localhost:3000` muestra el header con el selector poblado con los tres usuarios. Como todavía no existen las rutas destino, la redirección da 404: es lo esperado en este punto.

- [ ] **Step 11: Commit**

```bash
git add web
git commit -m "feat(web): esqueleto Next.js, cliente del API y selector de usuario"
```

---

## Task 23: Frontend — vista del colaborador

**Files:**
- Create: `web/app/mis-vacaciones/page.tsx`, `web/components/FormularioSolicitud.tsx`, `web/components/TablaMovimientos.tsx`

- [ ] **Step 1: Crear `web/components/TablaMovimientos.tsx`**

Se reutiliza en la vista de RRHH, así que se escribe una sola vez.

```tsx
import { Etiqueta, fecha } from "./ui";
import type { Movimiento } from "@/lib/tipos";

export function TablaMovimientos({ movimientos }: { movimientos: Movimiento[] }) {
  if (movimientos.length === 0) {
    return <p className="text-sm text-slate-500">Sin movimientos.</p>;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
            <th className="py-2 pr-4">Fecha</th>
            <th className="py-2 pr-4">Clase</th>
            <th className="py-2 pr-4">Tipo</th>
            <th className="py-2 pr-4 text-right">Días</th>
            <th className="py-2 pr-4">Motivo</th>
            <th className="py-2">Actor</th>
          </tr>
        </thead>
        <tbody>
          {movimientos.map((m) => {
            const negativo = m.Cantidad.startsWith("-");
            return (
              <tr key={m.ID} className="border-b border-slate-100 align-top">
                <td className="py-2 pr-4 whitespace-nowrap font-mono text-xs">
                  {fecha(m.FechaEfectiva)}
                </td>
                <td className="py-2 pr-4">
                  <Etiqueta valor={m.Clase} />
                </td>
                <td className="py-2 pr-4 whitespace-nowrap text-slate-600">{m.TipoCodigo}</td>
                <td
                  className={`py-2 pr-4 text-right font-mono ${
                    negativo ? "text-rose-700" : "text-emerald-700"
                  }`}
                >
                  {negativo ? m.Cantidad : `+${m.Cantidad}`}
                </td>
                <td className="py-2 pr-4 text-slate-600">{m.Motivo}</td>
                <td className="py-2 whitespace-nowrap text-slate-500">{m.ActorNombre}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
```

- [ ] **Step 2: Crear `web/components/FormularioSolicitud.tsx`**

```tsx
"use client";

import { useEffect, useState } from "react";
import { pedir, postear } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Aviso } from "./ui";
import type { Preview, SaldoDeTipo } from "@/lib/tipos";

export function FormularioSolicitud({
  tipos,
  alCrear,
}: {
  tipos: SaldoDeTipo[];
  alCrear: () => void;
}) {
  const { actor } = useActor();
  const [tipoId, setTipoId] = useState(tipos[0]?.tipo_id ?? "");
  const [desde, setDesde] = useState("");
  const [hasta, setHasta] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");
  const [enviando, setEnviando] = useState(false);

  // El preview lo calcula el API, no el navegador: los días hábiles dependen
  // del calendario de feriados, que vive en la base.
  useEffect(() => {
    if (!desde || !hasta || hasta < desde || !actor) {
      setPreview(null);
      return;
    }
    pedir<Preview>(`/api/solicitudes/preview?desde=${desde}&hasta=${hasta}`, actor.id)
      .then(setPreview)
      .catch(() => setPreview(null));
  }, [desde, hasta, actor]);

  async function enviar(e: React.FormEvent) {
    e.preventDefault();
    if (!actor) return;

    setEnviando(true);
    setError("");
    setOk("");
    try {
      await postear("/api/solicitudes", actor.id, { tipo_id: tipoId, desde, hasta });
      setOk("Solicitud creada. Queda pendiente de aprobación.");
      setDesde("");
      setHasta("");
      setPreview(null);
      alCrear();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido");
    } finally {
      setEnviando(false);
    }
  }

  const seleccionado = tipos.find((t) => t.tipo_id === tipoId);

  return (
    <form onSubmit={enviar} className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-3">
        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Tipo</span>
          <select
            value={tipoId}
            onChange={(e) => setTipoId(e.target.value)}
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          >
            {tipos.map((t) => (
              <option key={t.tipo_id} value={t.tipo_id}>
                {t.tipo_nombre || t.tipo_codigo}
              </option>
            ))}
          </select>
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Desde</span>
          <input
            type="date"
            value={desde}
            onChange={(e) => setDesde(e.target.value)}
            required
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          />
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Hasta</span>
          <input
            type="date"
            value={hasta}
            onChange={(e) => setHasta(e.target.value)}
            required
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          />
        </label>
      </div>

      {preview && (
        <p className="text-sm text-slate-600">
          Descuenta{" "}
          <strong className="font-mono">{preview.dias_habiles}</strong> días hábiles
          de {preview.dias_corridos} corridos.
          {seleccionado && (
            <>
              {" "}Tienes <strong className="font-mono">{seleccionado.solicitable}</strong>{" "}
              solicitables.
            </>
          )}
        </p>
      )}

      {error && <Aviso mensaje={error} />}
      {ok && <Aviso mensaje={ok} tono="ok" />}

      <button
        type="submit"
        disabled={enviando || !preview}
        className="rounded bg-slate-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-40"
      >
        {enviando ? "Enviando…" : "Solicitar"}
      </button>
    </form>
  );
}
```

- [ ] **Step 3: Crear `web/app/mis-vacaciones/page.tsx`**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { pedir } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, Etiqueta, fecha } from "@/components/ui";
import { FormularioSolicitud } from "@/components/FormularioSolicitud";
import { TablaMovimientos } from "@/components/TablaMovimientos";
import type { Movimiento, Saldo, Solicitud } from "@/lib/tipos";

export default function MisVacaciones() {
  const { actor, cargando } = useActor();
  const [saldo, setSaldo] = useState<Saldo | null>(null);
  const [solicitudes, setSolicitudes] = useState<Solicitud[]>([]);
  const [movimientos, setMovimientos] = useState<Movimiento[]>([]);

  const recargar = useCallback(() => {
    if (!actor) return;
    pedir<Saldo>(`/api/colaboradores/${actor.id}/saldo`, actor.id).then(setSaldo);
    pedir<Solicitud[]>("/api/solicitudes", actor.id).then(setSolicitudes);
    pedir<Movimiento[]>(`/api/colaboradores/${actor.id}/movimientos`, actor.id).then(
      setMovimientos,
    );
  }, [actor]);

  useEffect(recargar, [recargar]);

  if (cargando || !actor) return <p className="text-slate-500">Cargando…</p>;

  const tipos = saldo?.por_tipo ?? [];
  const porVencer = tipos.flatMap((t) =>
    (t.por_vencer ?? []).map((b) => ({ ...b, tipo: t.tipo_nombre || t.tipo_codigo })),
  );

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Hola, {actor.nombre}</h1>
        <p className="text-sm text-slate-500">
          Tienes <strong className="font-mono">{saldo?.total_disponible ?? "—"}</strong> días
          disponibles en total.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {tipos.map((t) => (
          <Tarjeta key={t.tipo_id} titulo={t.tipo_nombre || t.tipo_codigo}>
            <p className="font-mono text-3xl">{t.disponible}</p>
            <p className="mt-1 text-xs text-slate-500">días disponibles</p>
            {t.pendiente !== "0" && (
              <p className="mt-2 text-xs text-amber-700">
                {t.pendiente} comprometidos en solicitudes pendientes ·{" "}
                <strong>{t.solicitable}</strong> solicitables
              </p>
            )}
          </Tarjeta>
        ))}
      </div>

      {porVencer.length > 0 && (
        <Tarjeta titulo="Próximos a vencer">
          <ul className="space-y-1 text-sm">
            {porVencer.map((b) => (
              <li key={b.OtorgamientoID} className="flex justify-between">
                <span className="text-slate-600">{b.tipo}</span>
                <span>
                  <strong className="font-mono">{b.Dias}</strong> días vencen el{" "}
                  <span className="font-mono">{fecha(b.VenceEl)}</span>
                </span>
              </li>
            ))}
          </ul>
        </Tarjeta>
      )}

      <Tarjeta titulo="Solicitar vacaciones">
        {tipos.length > 0 ? (
          <FormularioSolicitud tipos={tipos} alCrear={recargar} />
        ) : (
          <p className="text-sm text-slate-500">No tienes saldo disponible para solicitar.</p>
        )}
      </Tarjeta>

      <Tarjeta titulo="Mis solicitudes">
        {solicitudes.length === 0 ? (
          <p className="text-sm text-slate-500">Sin solicitudes.</p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
                <th className="py-2 pr-4">Desde</th>
                <th className="py-2 pr-4">Hasta</th>
                <th className="py-2 pr-4">Tipo</th>
                <th className="py-2 pr-4 text-right">Días hábiles</th>
                <th className="py-2">Estado</th>
              </tr>
            </thead>
            <tbody>
              {solicitudes.map((s) => (
                <tr key={s.ID} className="border-b border-slate-100">
                  <td className="py-2 pr-4 font-mono text-xs">{fecha(s.Desde)}</td>
                  <td className="py-2 pr-4 font-mono text-xs">{fecha(s.Hasta)}</td>
                  <td className="py-2 pr-4 text-slate-600">{s.TipoCodigo}</td>
                  <td className="py-2 pr-4 text-right font-mono">{s.DiasHabiles}</td>
                  <td className="py-2">
                    <Etiqueta valor={s.Estado} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Tarjeta>

      <Tarjeta titulo="Mi historial">
        <TablaMovimientos movimientos={movimientos} />
      </Tarjeta>
    </div>
  );
}
```

- [ ] **Step 4: Verificar en el navegador**

Con `make api` y `make web` corriendo, abrir `http://localhost:3000` con **Ana** seleccionada.

Expected: cards de saldo, una solicitud pendiente listada, y el historial con sus `ACCRUAL` y `CONSUMPTION`.

Cambiar el selector a **Carlos**.

Expected: su historial muestra los `ACCRUAL` de 15, el de 17 y el de 18, más los `EXPIRATION`.

- [ ] **Step 5: Commit**

```bash
git add web/app/mis-vacaciones web/components
git commit -m "feat(web): vista del colaborador con saldo, solicitud e historial"
```

---

## Task 24: Frontend — vista de RRHH: colaboradores y detalle

**Files:**
- Create: `web/app/rrhh/page.tsx`, `web/app/rrhh/colaboradores/[id]/page.tsx`, `web/components/FormularioOtorgamiento.tsx`

- [ ] **Step 1: Crear `web/app/rrhh/page.tsx`**

```tsx
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { pedir } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, fecha } from "@/components/ui";
import type { ColaboradorConSaldo } from "@/lib/tipos";

export default function Colaboradores() {
  const { actor, cargando } = useActor();
  const [filas, setFilas] = useState<ColaboradorConSaldo[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!actor) return;
    pedir<ColaboradorConSaldo[]>("/api/colaboradores", actor.id)
      .then(setFilas)
      .catch((e) => setError(e.message));
  }, [actor]);

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (actor?.rol !== "RRHH") {
    return <p className="text-slate-500">Esta vista requiere rol de RRHH.</p>;
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Colaboradores</h1>
      {error && <p className="text-sm text-rose-700">{error}</p>}

      <Tarjeta>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-4">Nombre</th>
              <th className="py-2 pr-4">Ingreso</th>
              <th className="py-2 pr-4 text-right">Antigüedad</th>
              <th className="py-2 pr-4 text-right">Disponible</th>
              <th className="py-2"></th>
            </tr>
          </thead>
          <tbody>
            {filas.map((c) => (
              <tr key={c.id} className="border-b border-slate-100">
                <td className="py-2 pr-4">
                  <span className="font-medium">{c.nombre}</span>
                  <span className="ml-2 text-xs text-slate-400">{c.email}</span>
                </td>
                <td className="py-2 pr-4 font-mono text-xs">{fecha(c.fecha_ingreso)}</td>
                <td className="py-2 pr-4 text-right">{c.anios_antiguedad} años</td>
                <td className="py-2 pr-4 text-right font-mono">{c.disponible}</td>
                <td className="py-2 text-right">
                  <Link
                    href={`/rrhh/colaboradores/${c.id}`}
                    className="text-sm text-slate-600 underline hover:text-slate-900"
                  >
                    Ver detalle
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Tarjeta>
    </div>
  );
}
```

- [ ] **Step 2: Crear `web/components/FormularioOtorgamiento.tsx`**

```tsx
"use client";

import { useEffect, useState } from "react";
import { pedir, postear } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Aviso } from "./ui";
import type { TipoDeVacacion } from "@/lib/tipos";

export function FormularioOtorgamiento({
  colaboradorId,
  alOtorgar,
}: {
  colaboradorId: string;
  alOtorgar: () => void;
}) {
  const { actor } = useActor();
  const [tipos, setTipos] = useState<TipoDeVacacion[]>([]);
  const [tipoId, setTipoId] = useState("");
  const [dias, setDias] = useState("");
  const [motivo, setMotivo] = useState("");
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");

  useEffect(() => {
    if (!actor) return;
    pedir<TipoDeVacacion[]>("/api/tipos-vacacion", actor.id).then((lista) => {
      setTipos(lista);
      setTipoId((actual) => actual || lista[0]?.ID || "");
    });
  }, [actor]);

  async function enviar(e: React.FormEvent) {
    e.preventDefault();
    if (!actor) return;

    setError("");
    setOk("");
    try {
      await postear("/api/otorgamientos", actor.id, {
        colaborador_id: colaboradorId,
        tipo_id: tipoId,
        dias,
        motivo,
      });
      setOk("Días otorgados. Quedó registrado en el historial con tu nombre.");
      setDias("");
      setMotivo("");
      alOtorgar();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido");
    }
  }

  return (
    <form onSubmit={enviar} className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-3">
        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Tipo</span>
          <select
            value={tipoId}
            onChange={(e) => setTipoId(e.target.value)}
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          >
            {tipos.map((t) => (
              <option key={t.ID} value={t.ID}>
                {t.Nombre}
              </option>
            ))}
          </select>
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-600">Días</span>
          <input
            value={dias}
            onChange={(e) => setDias(e.target.value)}
            placeholder="2.5"
            required
            className="w-full rounded border border-slate-300 px-2 py-1.5 font-mono"
          />
        </label>

        <label className="block text-sm sm:col-span-1">
          <span className="mb-1 block text-slate-600">Motivo (obligatorio)</span>
          <input
            value={motivo}
            onChange={(e) => setMotivo(e.target.value)}
            placeholder="Reconocimiento por proyecto Q3"
            required
            className="w-full rounded border border-slate-300 px-2 py-1.5"
          />
        </label>
      </div>

      {error && <Aviso mensaje={error} />}
      {ok && <Aviso mensaje={ok} tono="ok" />}

      <button
        type="submit"
        className="rounded bg-slate-900 px-4 py-2 text-sm font-medium text-white"
      >
        Otorgar días
      </button>
    </form>
  );
}
```

- [ ] **Step 3: Crear `web/app/rrhh/colaboradores/[id]/page.tsx`**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { pedir } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, fecha } from "@/components/ui";
import { TablaMovimientos } from "@/components/TablaMovimientos";
import { FormularioOtorgamiento } from "@/components/FormularioOtorgamiento";
import type { Finiquito, Movimiento, Saldo } from "@/lib/tipos";

export default function DetalleColaborador() {
  const { actor, cargando } = useActor();
  const params = useParams<{ id: string }>();
  const id = params.id;

  const [saldo, setSaldo] = useState<Saldo | null>(null);
  const [movimientos, setMovimientos] = useState<Movimiento[]>([]);
  const [finiquito, setFiniquito] = useState<Finiquito | null>(null);
  const [corte, setCorte] = useState("");

  const recargar = useCallback(() => {
    if (!actor || actor.rol !== "RRHH") return;
    pedir<Saldo>(`/api/colaboradores/${id}/saldo`, actor.id).then(setSaldo);
    pedir<Finiquito>(`/api/colaboradores/${id}/finiquito`, actor.id).then(setFiniquito);
    const sufijo = corte ? `?hasta=${corte}` : "";
    pedir<Movimiento[]>(`/api/colaboradores/${id}/movimientos${sufijo}`, actor.id).then(
      setMovimientos,
    );
  }, [actor, id, corte]);

  useEffect(recargar, [recargar]);

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (actor?.rol !== "RRHH") {
    return <p className="text-slate-500">Esta vista requiere rol de RRHH.</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{saldo?.colaborador_nombre ?? "…"}</h1>
        <p className="text-sm text-slate-500">
          Ingreso {saldo ? fecha(saldo.fecha_ingreso) : "—"}
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {(saldo?.por_tipo ?? []).map((t) => (
          <Tarjeta key={t.tipo_id} titulo={t.tipo_nombre || t.tipo_codigo}>
            <p className="font-mono text-3xl">{t.disponible}</p>
            <p className="mt-1 text-xs text-slate-500">disponibles</p>
          </Tarjeta>
        ))}

        {saldo?.devengado_no_otorgado && (
          <Tarjeta titulo="Devengado no otorgado">
            <p className="font-mono text-3xl">{saldo.devengado_no_otorgado}</p>
            <p className="mt-1 text-xs text-slate-500">
              del período en curso · solo visible para RRHH
            </p>
          </Tarjeta>
        )}
      </div>

      {finiquito && (
        <Tarjeta titulo="Si se desvincula hoy">
          <div className="grid gap-4 text-sm sm:grid-cols-3">
            <div>
              <p className="text-slate-500">Proporcional del período</p>
              <p className="font-mono text-2xl">{finiquito.Proporcional.Dias}</p>
              <p className="mt-1 text-xs text-slate-500">
                {finiquito.Proporcional.MesesCompletos} meses × 1,25 + (
                {finiquito.Proporcional.DiasRestantes}/30) × 1,25
              </p>
            </div>
            <div>
              <p className="text-slate-500">Disponible pagable</p>
              <p className="font-mono text-2xl">{finiquito.DisponiblePagable}</p>
              <p className="mt-1 text-xs text-slate-500">
                solo tipos marcados como pagables en finiquito
              </p>
            </div>
            <div>
              <p className="text-slate-500">Total a pagar</p>
              <p className="font-mono text-2xl font-semibold">{finiquito.Total}</p>
              <p className="mt-1 text-xs text-slate-500">días hábiles</p>
            </div>
          </div>
        </Tarjeta>
      )}

      <Tarjeta titulo="Otorgar saldo especial">
        <FormularioOtorgamiento colaboradorId={id} alOtorgar={recargar} />
      </Tarjeta>

      <Tarjeta titulo="Historial de movimientos">
        <label className="mb-4 flex items-center gap-2 text-sm">
          <span className="text-slate-600">Reconstruir el saldo a la fecha</span>
          <input
            type="date"
            value={corte}
            onChange={(e) => setCorte(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1"
          />
          {corte && (
            <button
              onClick={() => setCorte("")}
              className="text-slate-500 underline hover:text-slate-900"
            >
              ver todo
            </button>
          )}
        </label>
        <TablaMovimientos movimientos={movimientos} />
      </Tarjeta>
    </div>
  );
}
```

- [ ] **Step 4: Verificar en el navegador**

Con **Marta** seleccionada, abrir `http://localhost:3000/rrhh` y entrar al detalle de Carlos.

Expected: se ve el card de "Devengado no otorgado" (que Ana no ve en su propia vista), el desglose del finiquito con la fórmula, y el historial completo. Poner una fecha de corte en el pasado recorta el historial.

- [ ] **Step 5: Commit**

```bash
git add web/app/rrhh web/components/FormularioOtorgamiento.tsx
git commit -m "feat(web): vista RRHH con detalle, finiquito y otorgamiento manual"
```

---

## Task 25: Frontend — bandeja de solicitudes y ABM de tipos

**Files:**
- Create: `web/app/rrhh/solicitudes/page.tsx`, `web/app/rrhh/tipos/page.tsx`

- [ ] **Step 1: Crear `web/app/rrhh/solicitudes/page.tsx`**

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { pedir, postear } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, Etiqueta, Aviso, fecha } from "@/components/ui";
import type { Solicitud } from "@/lib/tipos";

export default function Solicitudes() {
  const { actor, cargando } = useActor();
  const [solicitudes, setSolicitudes] = useState<Solicitud[]>([]);
  const [error, setError] = useState("");

  const recargar = useCallback(() => {
    if (!actor || actor.rol !== "RRHH") return;
    pedir<Solicitud[]>("/api/solicitudes", actor.id).then(setSolicitudes);
  }, [actor]);

  useEffect(recargar, [recargar]);

  async function decidir(id: string, accion: "aprobar" | "rechazar") {
    if (!actor) return;
    setError("");
    try {
      await postear(`/api/solicitudes/${id}/${accion}`, actor.id);
      recargar();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido");
    }
  }

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (actor?.rol !== "RRHH") {
    return <p className="text-slate-500">Esta vista requiere rol de RRHH.</p>;
  }

  const pendientes = solicitudes.filter((s) => s.Estado === "PENDIENTE");
  const decididas = solicitudes.filter((s) => s.Estado !== "PENDIENTE");

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Solicitudes</h1>
      {error && <Aviso mensaje={error} />}

      <Tarjeta titulo={`Pendientes (${pendientes.length})`}>
        {pendientes.length === 0 ? (
          <p className="text-sm text-slate-500">No hay solicitudes pendientes.</p>
        ) : (
          <ul className="divide-y divide-slate-100">
            {pendientes.map((s) => (
              <li key={s.ID} className="flex items-center justify-between py-3">
                <div className="text-sm">
                  <p className="font-medium">{s.ColaboradorNom}</p>
                  <p className="text-slate-500">
                    <span className="font-mono">{fecha(s.Desde)}</span> al{" "}
                    <span className="font-mono">{fecha(s.Hasta)}</span> ·{" "}
                    <strong className="font-mono">{s.DiasHabiles}</strong> días hábiles ·{" "}
                    {s.TipoCodigo}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => decidir(s.ID, "aprobar")}
                    className="rounded bg-emerald-600 px-3 py-1.5 text-sm font-medium text-white"
                  >
                    Aprobar
                  </button>
                  <button
                    onClick={() => decidir(s.ID, "rechazar")}
                    className="rounded border border-slate-300 px-3 py-1.5 text-sm"
                  >
                    Rechazar
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-4 text-xs text-slate-500">
          Al aprobar, el consumo se reparte entre las bolsas del tipo empezando por la que
          vence antes, y cada tramo genera su propio movimiento en el historial.
        </p>
      </Tarjeta>

      <Tarjeta titulo="Historial de solicitudes">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-4">Colaborador</th>
              <th className="py-2 pr-4">Desde</th>
              <th className="py-2 pr-4">Hasta</th>
              <th className="py-2 pr-4 text-right">Días</th>
              <th className="py-2">Estado</th>
            </tr>
          </thead>
          <tbody>
            {decididas.map((s) => (
              <tr key={s.ID} className="border-b border-slate-100">
                <td className="py-2 pr-4">{s.ColaboradorNom}</td>
                <td className="py-2 pr-4 font-mono text-xs">{fecha(s.Desde)}</td>
                <td className="py-2 pr-4 font-mono text-xs">{fecha(s.Hasta)}</td>
                <td className="py-2 pr-4 text-right font-mono">{s.DiasHabiles}</td>
                <td className="py-2">
                  <Etiqueta valor={s.Estado} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Tarjeta>
    </div>
  );
}
```

- [ ] **Step 2: Crear `web/app/rrhh/tipos/page.tsx`**

Esta pantalla es la demostración del Requisito 7. El formulario viene precargado con "días por rendimiento" para que crearlo en vivo sea un clic.

```tsx
"use client";

import { useCallback, useEffect, useState } from "react";
import { pedir, postear } from "@/lib/api";
import { useActor } from "@/lib/actor";
import { Tarjeta, Aviso } from "@/components/ui";
import type { Parametros, TipoDeVacacion } from "@/lib/tipos";

const NUEVO_POR_DEFECTO = {
  codigo: "RENDIMIENTO",
  nombre: "Días por rendimiento",
  politica_devengo: "manual",
  politica_vencimiento: "dias_fijos",
  prioridad_consumo: 5,
  unidad_habil: true,
  pagable_en_finiquito: false,
  parametros: { dias_fijos: 180 } as Parametros,
};

export default function Tipos() {
  const { actor, cargando } = useActor();
  const [tipos, setTipos] = useState<TipoDeVacacion[]>([]);
  const [nuevo, setNuevo] = useState(NUEVO_POR_DEFECTO);
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");

  const recargar = useCallback(() => {
    if (!actor) return;
    pedir<TipoDeVacacion[]>("/api/tipos-vacacion", actor.id).then(setTipos);
  }, [actor]);

  useEffect(recargar, [recargar]);

  async function crear(e: React.FormEvent) {
    e.preventDefault();
    if (!actor) return;

    setError("");
    setOk("");
    try {
      await postear("/api/tipos-vacacion", actor.id, nuevo);
      setOk(`Tipo "${nuevo.nombre}" creado. Ya se puede otorgar, sin desplegar código.`);
      recargar();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido");
    }
  }

  if (cargando) return <p className="text-slate-500">Cargando…</p>;
  if (actor?.rol !== "RRHH") {
    return <p className="text-slate-500">Esta vista requiere rol de RRHH.</p>;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Tipos de vacación</h1>
        <p className="text-sm text-slate-500">
          Cada tipo compone tres políticas intercambiables. Agregar uno nuevo es crear un
          registro, no escribir código.
        </p>
      </div>

      <Tarjeta titulo="Configurados">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-4">Código</th>
              <th className="py-2 pr-4">Devengo</th>
              <th className="py-2 pr-4">Vencimiento</th>
              <th className="py-2 pr-4 text-right">Prioridad</th>
              <th className="py-2 pr-4">Pagable</th>
              <th className="py-2">Parámetros</th>
            </tr>
          </thead>
          <tbody>
            {tipos.map((t) => (
              <tr key={t.ID} className="border-b border-slate-100">
                <td className="py-2 pr-4 font-medium">{t.Codigo}</td>
                <td className="py-2 pr-4 font-mono text-xs">{t.PoliticaDevengo}</td>
                <td className="py-2 pr-4 font-mono text-xs">{t.PoliticaVencimiento}</td>
                <td className="py-2 pr-4 text-right">{t.PrioridadConsumo}</td>
                <td className="py-2 pr-4">{t.PagableEnFiniquito ? "sí" : "no"}</td>
                <td className="py-2 font-mono text-xs text-slate-500">
                  {JSON.stringify(t.Parametros)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Tarjeta>

      <Tarjeta titulo="Crear un tipo nuevo">
        <form onSubmit={crear} className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <label className="block text-sm">
              <span className="mb-1 block text-slate-600">Código</span>
              <input
                value={nuevo.codigo}
                onChange={(e) => setNuevo({ ...nuevo, codigo: e.target.value })}
                className="w-full rounded border border-slate-300 px-2 py-1.5 font-mono"
              />
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-600">Nombre</span>
              <input
                value={nuevo.nombre}
                onChange={(e) => setNuevo({ ...nuevo, nombre: e.target.value })}
                className="w-full rounded border border-slate-300 px-2 py-1.5"
              />
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-600">Devengo</span>
              <select
                value={nuevo.politica_devengo}
                onChange={(e) => setNuevo({ ...nuevo, politica_devengo: e.target.value })}
                className="w-full rounded border border-slate-300 px-2 py-1.5"
              >
                <option value="manual">manual</option>
                <option value="aniversario_legal">aniversario_legal</option>
                <option value="anio_calendario">anio_calendario</option>
              </select>
            </label>

            <label className="block text-sm">
              <span className="mb-1 block text-slate-600">Vencimiento</span>
              <select
                value={nuevo.politica_vencimiento}
                onChange={(e) =>
                  setNuevo({ ...nuevo, politica_vencimiento: e.target.value })
                }
                className="w-full rounded border border-slate-300 px-2 py-1.5"
              >
                <option value="dias_fijos">dias_fijos</option>
                <option value="fin_de_anio">fin_de_anio</option>
                <option value="n_periodos">n_periodos</option>
                <option value="no_vence">no_vence</option>
              </select>
            </label>
          </div>

          <label className="block text-sm">
            <span className="mb-1 block text-slate-600">
              Parámetros (JSON: dias_base, dias_fijos, n_periodos…)
            </span>
            <input
              value={JSON.stringify(nuevo.parametros)}
              onChange={(e) => {
                try {
                  setNuevo({ ...nuevo, parametros: JSON.parse(e.target.value) });
                  setError("");
                } catch {
                  setError("Los parámetros deben ser JSON válido");
                }
              }}
              className="w-full rounded border border-slate-300 px-2 py-1.5 font-mono text-xs"
            />
          </label>

          {error && <Aviso mensaje={error} />}
          {ok && <Aviso mensaje={ok} tono="ok" />}

          <button
            type="submit"
            className="rounded bg-slate-900 px-4 py-2 text-sm font-medium text-white"
          >
            Crear tipo
          </button>
        </form>
      </Tarjeta>
    </div>
  );
}
```

- [ ] **Step 3: Verificar el flujo completo en el navegador**

1. Con **Marta**, ir a Tipos y crear "Días por rendimiento" con los valores precargados.

Expected: aparece en la tabla de configurados.

2. Ir al detalle de **Ana** y otorgarle 2 días de ese tipo nuevo, con motivo.

Expected: el historial muestra un `ACCRUAL` de `+2` con el motivo y "Marta Silva" como actor.

3. Cambiar el selector a **Ana**.

Expected: aparece un card nuevo de "Días por rendimiento" con 2 días disponibles.

4. Con Ana, solicitar vacaciones. Con Marta, aprobarlas.

Expected: el historial de Ana muestra uno o más `CONSUMPTION` negativos y su saldo baja.

- [ ] **Step 4: Commit**

```bash
git add web/app/rrhh
git commit -m "feat(web): bandeja de solicitudes y ABM de tipos de vacación"
```

---

## Task 26: README

El README es parte del entregable, no un extra. Tiene que explicar la solución de forma autónoma: alguien que abre el repositorio sin contexto debe entender qué problema resuelve, por qué está construido así, y cómo correrlo.

**Files:**
- Create: `README.md`

- [ ] **Step 1: Escribir `README.md`**

Escribir exactamente este contenido, ajustando los uuid de ejemplo por los que devuelva la semilla:

````markdown
# Gestión de Vacaciones

Sistema de gestión de saldos de vacaciones donde **el saldo es un dato derivado, no almacenado**.

## El problema

La gestión de vacaciones suele resolverse con una planilla donde el saldo es una columna que alguien debe acordarse de actualizar. Eso produce cuatro problemas que aparecen siempre juntos:

- **Deriva de datos.** El número deja de reflejar la realidad y nadie sabe cuándo empezó.
- **Falta de auditoría.** No se puede reconstruir cómo se llegó a un saldo, ni quién lo cambió.
- **Sin autoservicio.** Cada consulta de un colaborador es una interrupción para RRHH.
- **Reglas en el código.** Agregar un tipo de vacación nuevo exige desarrollo y despliegue.

Migrar esa planilla a una base de datos, manteniendo el saldo como una columna mutable, reproduce el mismo problema — solo que más rápido.

## La decisión central

Este sistema aplica el principio de la partida doble: **cada evento que afecta las vacaciones se escribe como un movimiento inmutable, y el saldo es su suma**.

No existe ninguna columna `saldo` en ninguna tabla. Cuando el sistema responde "tienes 18 días disponibles", ese número se acaba de calcular sumando el historial. Un número que no existe no puede quedar desactualizado.

La consecuencia práctica es que la auditoría, la reportería y la explicabilidad salen del modelo de datos en lugar de ser tres módulos aparte. El historial no es una funcionalidad que se agrega después: es la estructura de datos.

### Tres conceptos que el negocio suele mezclar

| Concepto | Qué es | Quién lo necesita |
|---|---|---|
| **Devengado** | Derecho que se gana de forma continua, a razón de 1,25 días hábiles por mes. Existe desde el día 1. | RRHH, para calcular un finiquito |
| **Otorgado / disponible** | Derecho habilitado para uso. El devengado se convierte en otorgado en el aniversario de ingreso. | El colaborador |
| **Consumido / vencido** | Movimientos negativos que reducen el disponible. | Ambos |

Separarlos permite responder con un mismo modelo dos preguntas distintas: *"¿cuánto puedo tomar hoy?"* y *"¿cuánto le debo si lo desvinculo hoy?"*.

### Las reglas son datos, no código

Un tipo de vacación no es una rama condicional en el código: es un registro que compone tres políticas intercambiables.

| Tipo | Devengo | Vencimiento | Unidad | Pagable |
|---|---|---|---|---|
| Feriado legal | aniversario legal + progresivo | 2 períodos | hábil | sí |
| Días administrativos | año calendario | fin de año calendario | hábil | no |
| Días por rendimiento | manual | 180 días fijos | hábil | no |

El mismo motor, distinta configuración. Agregar "días por rendimiento" es crear un registro desde la interfaz de RRHH, no escribir código ni desplegar.

## Reglas de cálculo implementadas

Todas en aritmética decimal de precisión fija. No hay `float` en ningún punto del cálculo, ni en el servidor ni en el navegador: los decimales viajan en JSON como string precisamente para que el frontend no los convierta.

**Feriado legal.** 15 días hábiles por aniversario de la fecha de ingreso.

**Feriado progresivo.** Si la experiencia total acreditada alcanza 120 meses y la antigüedad con el empleador actual alcanza 36 meses, se suma `floor(años_con_empleador / 3)` días. Umbral, antigüedad mínima, cadencia y días por tramo son parámetros de la política, no constantes del código: un cambio normativo no obliga a tocar el motor.

**Feriado proporcional.** Sobre el período en curso desde el último aniversario:

```
meses_completos × 1,25 + (días_restantes / 30) × 1,25
```

**Días hábiles.** Se descuentan domingos, sábados (el sábado es día inhábil por regla) y los feriados del calendario laboral.

**Consumo FIFO.** Se descuentan primero los días que vencen antes, y a igualdad de fecha, por prioridad del tipo. Una solicitud puede repartirse entre varias bolsas, y **cada tramo genera su propio movimiento**, de modo que el historial muestra de qué otorgamiento salió cada día.

**Vencimiento.** Al llegar la fecha, un movimiento `EXPIRATION` por el remanente exacto de la bolsa.

## Arquitectura

```
cmd/api        arranque del servidor HTTP
cmd/migrate    aplicación del esquema
cmd/seed       siembra, corriendo el motor sobre la historia
internal/
  domain/      reglas de negocio. Sin imports de pgx ni de net/http
  app/         casos de uso, orquestación y transacciones
  store/       repositorios y migraciones SQL
  http/        router, middleware de actor y handlers
web/           frontend Next.js
```

La regla estructural que sostiene todo lo demás: **`internal/domain` no importa infraestructura**. Las reglas de cálculo se testean sin base de datos y sin servidor, y se pueden leer completas sin atravesar capas.

**Stack:** Go 1.25 con el router de la biblioteca estándar, PostgreSQL 16, `pgx/v5`, `shopspring/decimal`, Next.js con TypeScript y Tailwind. Tres dependencias en el servidor, deliberadamente.

### Tres decisiones de persistencia

**La inmutabilidad la impone la base de datos.** El rol de aplicación recibe `GRANT INSERT, SELECT` sobre la tabla de movimientos y nada más. Un `UPDATE` sobre el ledger falla en el motor de base de datos, aunque alguien lo escriba por error en el código:

```bash
docker compose exec -T postgres psql -U vacaciones_app -d vacaciones \
  -c "UPDATE movimiento SET cantidad = 0;"
# ERROR:  permission denied for table movimiento
```

**La fecha de vencimiento se persiste, no se recalcula.** La política la calcula al momento del otorgamiento y se guarda. Si la política cambia después, el saldo pasado sigue siendo explicable con las reglas que estaban vigentes cuando se otorgó.

**Cada movimiento es bitemporal.** Guarda cuándo ocurrió el hecho (`fecha_efectiva`) y cuándo lo supimos (`fecha_registro`). Eso permite responder "¿qué sabíamos el 3 de marzo?" y no solo "¿qué es cierto hoy?".

## Cómo correrlo

Requiere Go 1.25, Node 20+ y Docker.

```bash
cp .env.example .env
make db-up      # levanta Postgres en localhost:5433
make migrate    # aplica el esquema y los permisos
make seed       # siembra los datos de demostración
make api        # API en http://localhost:8080
make web        # frontend en http://localhost:3000
```

Para empezar de cero en cualquier momento: `make demo-reset`.

Los tests del dominio no necesitan base de datos:

```bash
make test
```

## Datos de demostración

No hay login. Un selector en el header cambia el usuario activo, y el rol determina qué se ve.

| Usuario | Rol | Situación |
|---|---|---|
| **Ana Fuentes** | Colaboradora | Ingreso reciente. Caso simple, sin progresivas. |
| **Carlos Rojas** | Colaborador | Nueve años de antigüedad y experiencia previa acreditada. |
| **Marta Silva** | RRHH | Ve a todos, aprueba solicitudes, configura tipos. |

El historial de Carlos **no está insertado a mano**: la semilla recorre su historia completa corriendo el mismo motor que corre en producción. El ledger resultante es el que el sistema habría producido si hubiera estado funcionando desde su primer día.

Por eso su historial muestra el efecto de las progresivas en dos escalones:

| Aniversario | Años con el empleador | Experiencia total | Días otorgados |
|---|---|---|---|
| 1º a 7º | 1 – 7 | bajo el umbral | 15,00 |
| 8º | 8 | alcanza los 120 meses | **17,00** |
| 9º | 9 | — | **18,00** |

Junto a esos `ACCRUAL` aparecen los `EXPIRATION` de las bolsas que superaron el tope de dos períodos, y los `CONSUMPTION` de sus vacaciones, repartidos entre bolsas.

## API

Todas las llamadas llevan el header `X-Actor-Id`. El rol del actor determina qué datos se devuelven: un colaborador ve solo su disponible; RRHH ve además el devengado no otorgado y el proporcional de finiquito.

```
GET  /api/usuarios                               lista para el selector
GET  /api/colaboradores                          RRHH: todos, con su disponible
GET  /api/colaboradores/{id}/saldo               saldo proyectado desde el ledger
GET  /api/colaboradores/{id}/movimientos         historial; ?hasta= corta a una fecha pasada
GET  /api/colaboradores/{id}/finiquito?fecha=    proporcional, solo lectura
GET  /api/solicitudes                            propias, o todas si es RRHH
GET  /api/solicitudes/preview?desde=&hasta=      días hábiles del rango
POST /api/solicitudes                            crear
POST /api/solicitudes/{id}/aprobar               consumo FIFO transaccional
POST /api/solicitudes/{id}/rechazar
GET  /api/tipos-vacacion
POST /api/tipos-vacacion                         crear un tipo sin desplegar código
PUT  /api/tipos-vacacion/{id}
POST /api/otorgamientos                          carga manual, motivo obligatorio
POST /api/jobs/devengo?fecha=YYYY-MM-DD
POST /api/jobs/vencimiento?fecha=YYYY-MM-DD
```

### Los procesos automáticos

El devengo y el vencimiento son jobs. No tienen interfaz: se exponen como endpoints y en producción los dispararía un scheduler.

```bash
curl -X POST -H "X-Actor-Id: <uuid-de-rrhh>" \
  "http://localhost:8080/api/jobs/devengo?fecha=2026-04-15"
```

Dos propiedades los hacen operables:

**Son idempotentes.** Cada movimiento automático lleva una clave de idempotencia única en base de datos. Correr el job dos veces para la misma fecha no duplica nada: el segundo `INSERT` colisiona y se descarta. La respuesta lo reporta explícitamente en `ya_existian`.

**Aceptan fecha objetivo.** Si el job no corrió un día, se vuelve a correr con esa fecha y recupera exactamente lo que faltaba. Sin esto, una falla silenciosa deja a decenas de personas sin sus días y nadie se entera hasta que reclaman — que es el problema original, reproducido dentro del sistema nuevo.

## Alcance

Este es un MVP construido para demostrar el modelo, no un sistema de producción. Lo que está fuera, y por qué:

| Fuera de alcance | Nota |
|---|---|
| Snapshots de saldo y job de reconciliación | El saldo se suma del ledger en cada consulta. Con volumen real corresponde un snapshot consolidado más un job que lo compare contra el ledger y alerte ante diferencias. El contrato del API no cambiaría. |
| Multiempresa operativa | El `empresa_id` existe en todas las tablas y todos los repositorios filtran por él. Falta la interfaz y el aislamiento de credenciales. |
| Rol de jefatura | Aprueba RRHH. La máquina de estados y el campo de aprobador ya soportan otro rol: es cambiar quién puede llamar al endpoint. |
| Recálculo retroactivo | La primitiva existe — la clase `REVERSAL` y el campo `reversa_de` están implementados. Falta el proceso que reversa los movimientos automáticos derivados de un dato corregido y reejecuta el motor. |
| Versionado de políticas por fecha de vigencia | El campo `vigente_desde` existe, pero el motor no resuelve la versión aplicable según la fecha del cálculo. |
| Notificaciones de vencimiento | El saldo marca lo que está por vencer; no hay envío. |
| Monto en dinero del finiquito | Se calculan días, no pesos. |
| Licencias médicas, contratos por obra o faena, jornadas parciales | Reglas especiales no modeladas. |

El calendario laboral incluye solo los feriados de fecha fija. Los movibles se cargarían desde una fuente oficial en producción.
````

- [ ] **Step 2: Reemplazar los uuid de ejemplo**

En la sección de jobs, cambiar `<uuid-de-rrhh>` por el uuid real de Marta que imprimió la semilla, para que el comando sea copiable.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: README con la explicación completa de la solución"
```

---

## Task 27: Verificación final

**Files:** ninguno. Es la comprobación de que todo el plan quedó operativo.

- [ ] **Step 1: Reconstruir la demo desde cero**

```bash
make demo-reset
```

Expected: termina con `semilla lista`, sin errores.

- [ ] **Step 2: Correr la suite completa de tests**

```bash
make test
```

Expected: `ok github.com/BeSaavedram/vacation-calculator/internal/domain`, con todos los tests en PASS. En particular deben aparecer en PASS:
- `TestDevengo_NueveAniosDanDieciochoDias`
- `TestDevengo_ProgresivoEnLosBordes`
- `TestProporcional_CasoDeReferencia`
- `TestAsignarConsumo_RepartEntreVariasBolsas`

- [ ] **Step 3: Verificar que el ledger es realmente inmutable**

```bash
docker compose exec -T postgres psql -U vacaciones_app -d vacaciones -c "UPDATE movimiento SET cantidad = 0;"
```

Expected: `ERROR:  permission denied for table movimiento`

- [ ] **Step 4: Recorrer el guion de demo completo**

Con `make api` y `make web` corriendo:

1. Abrir con **Carlos**: ver su saldo y su historial con el salto 15 → 17 → 18.
2. Solicitar vacaciones con Carlos, ver el preview de días hábiles.
3. Cambiar a **Marta**: aprobar la solicitud en la bandeja.
4. Volver a **Carlos**: ver el saldo bajado y los `CONSUMPTION` repartidos en el historial.
5. Con **Marta**: crear el tipo "Días por rendimiento" desde Tipos.
6. Con **Marta**: otorgarle 2 días de ese tipo a **Ana** con un motivo.
7. Cambiar a **Ana**: ver el card nuevo y el movimiento con el motivo y el actor.
8. Con **Marta**: entrar al detalle de Carlos, mostrar el finiquito y poner una fecha de corte pasada en el historial.

Expected: los ocho pasos funcionan sin errores en consola del navegador ni del API.

- [ ] **Step 5: Verificar que el repositorio está limpio**

```bash
git status
```

Expected: sin cambios pendientes. Si `web/node_modules` aparece, agregarlo al `.gitignore` y commitear.

- [ ] **Step 6: Commit final**

```bash
git add .gitignore
git commit -m "chore: ignorar artefactos del frontend"
```

---

## Notas de la revisión del plan

Se revisó el plan contra el spec. Tres cosas que vale la pena tener presentes al ejecutar:

**Cobertura del spec.** Los diez requisitos de la propuesta tienen tarea asignada: R1 y R2 en el Task 6 y el Task 15; R3 en el Task 10; R4 y R9 en el Task 9 y el Task 16; R5 en el Task 8, el Task 12 y el historial con `?hasta=`; R6 en el Task 14 y el Task 23; R7 en el Task 17 y el Task 25; R8 en el Task 7 y el Task 15. R10 queda fuera de alcance de forma declarada, con la primitiva de reverso implementada.

**El campo `Prioridad` de `Bolsa` se agrega en el Task 9, no en el Task 8.** Si se ejecutan las tareas fuera de orden, `ordenarFIFO` no compilará hasta que ese campo exista.

**El seed depende de que el motor sea correcto.** El Task 20 incluye una verificación explícita de que los otorgamientos de Carlos den 17 y 18 en sus últimos dos aniversarios. Si ese chequeo falla, el problema está en el Task 6, no en la semilla: corregir ahí antes de seguir.

### Dos apartamientos del spec, deliberados

**El endpoint `GET /api/calendario` no se implementa.** El spec lo lista, pero ningún consumidor lo usa: el frontend obtiene los días hábiles de un rango desde `GET /api/solicitudes/preview`, que ya resuelve la misma pregunta con el cálculo del servidor. Implementarlo sería agregar superficie de API que nadie llama.

**El test de "vencimiento por remanente exacto tras un consumo parcial" no es un test de dominio.** El spec lo lista entre los tests unitarios, pero la lógica de vencimiento vive en `internal/app/jobs.go`, no en el dominio: necesita recorrer colaboradores y bolsas desde la base. Lo que sí queda cubierto por test unitario es el cálculo del remanente (`TestBolsaRemanente`, Task 8), que es la parte con reglas. El comportamiento completo del job se verifica contra datos reales en el Task 20 (Step 4) y en el Task 27.
