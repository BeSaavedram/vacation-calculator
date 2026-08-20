# Gestión de Vacaciones — Diseño del MVP

Fecha: 2026-08-20

## 1. Objetivo

Construir un MVP demostrable de un sistema de gestión de vacaciones, para presentar
visualmente una propuesta de solución. El MVP no busca cobertura funcional completa: busca
que la **tesis de la propuesta sea visible en pantalla**.

Esa tesis es una sola: *el saldo de vacaciones es un dato derivado, no almacenado*. Cada
evento que afecta las vacaciones se escribe como un movimiento inmutable, y el saldo es la
suma de esos movimientos. El historial no es una funcionalidad agregada después; es la
estructura de datos.

Todo lo demás en este documento existe para que ese punto se pueda demostrar en vivo.

### Criterios de éxito

La demo se considera exitosa si permite mostrar, sin tocar la base de datos a mano:

1. Un colaborador consulta su saldo disponible y sus días próximos a vencer.
2. Ese colaborador solicita vacaciones eligiendo el tipo, y ve el descuento de días hábiles.
3. RRHH aprueba la solicitud, y el consumo se reparte FIFO entre varias bolsas.
4. RRHH carga un saldo especial a un colaborador, con motivo obligatorio.
5. RRHH ve el historial completo de movimientos de un colaborador, y puede leer en él la
   historia de un empleado antiguo: sus devengos legales año a año y el momento exacto en
   que empieza a ganar días progresivos.
6. RRHH consulta cuánto le debe a un colaborador si lo desvincula hoy.
7. RRHH crea un tipo de vacación nuevo sin desplegar código.

## 2. Stack y forma de la aplicación

| Componente | Elección |
|---|---|
| API | Go 1.25, router de la stdlib (`net/http` con patrones de Go 1.22+) |
| Base de datos | PostgreSQL 16 en Docker, vía `docker-compose` |
| Acceso a datos | `pgx/v5` con SQL a mano. Sin ORM. |
| Aritmética | `shopspring/decimal`. Prohibido `float64` en cálculos de días. |
| Frontend | Next.js (App Router), TypeScript, Tailwind |
| Auth | Ninguna. Selector de usuario en el header. |

Dos procesos: `cmd/api` en `:8080` y Next.js en `:3000`. El FE llama al API por HTTP con un
header `X-Actor-Id`.

Las dependencias Go se mantienen al mínimo deliberadamente (`pgx`, `decimal` y nada más),
porque parte del valor de la demo es que el motor de cálculo se pueda leer completo.

### Estructura

```
vacation-calculator/
├─ cmd/
│  ├─ api/main.go              arranque del servidor
│  └─ seed/main.go             siembra de datos, corriendo el motor sobre la historia
├─ internal/
│  ├─ domain/                  PURO: sin imports de pgx ni de net/http
│  │  ├─ colaborador.go
│  │  ├─ tipo.go               TipoDeVacacion y la composición de sus políticas
│  │  ├─ politica_devengo.go   aniversario_legal · progresivo · año_calendario · manual
│  │  ├─ politica_vencimiento.go  no_vence · fin_de_año · n_periodos · dias_fijos
│  │  ├─ movimiento.go         el ledger y sus clases
│  │  ├─ saldo.go              proyección: []Movimiento → Saldo
│  │  ├─ fifo.go               AsignadorDeConsumo
│  │  ├─ habiles.go            ContadorDeDíasHábiles
│  │  └─ finiquito.go          feriado proporcional
│  ├─ app/                     casos de uso, orquestación y transacciones
│  ├─ store/                   repositorios pgx + migraciones SQL
│  └─ http/                    handlers, router, DTOs
├─ web/                        Next.js
├─ docker-compose.yml          solo Postgres
├─ Makefile
└─ README.md
```

La regla estructural que importa: `internal/domain` no importa nada de infraestructura. Las
reglas de negocio se testean sin base de datos y sin servidor.

## 3. Modelo de dominio

Tres conceptos que el negocio suele mezclar, y que aquí se mantienen separados:

- **Devengado** — derecho que se gana de forma continua, a razón de 15/12 = 1,25 días
  hábiles por mes. Existe desde el día 1.
- **Otorgado / disponible** — derecho habilitado para uso. El devengado se convierte en
  otorgado en el aniversario.
- **Consumido / vencido** — movimientos negativos que reducen el disponible.

Esta separación es la que permite responder con un mismo modelo las dos preguntas distintas
del negocio: *"¿cuánto puedo tomar hoy?"* (disponible, lo único que le interesa al
colaborador) y *"¿cuánto le debo si lo desvinculo hoy?"* (devengado del período en curso aún
no otorgado, lo que RRHH necesita para el finiquito).

### Las reglas son datos, no código

Un `TipoDeVacacion` no es una rama condicional: es un registro que compone políticas
intercambiables.

| Tipo | Devengo | Vencimiento | Unidad | Pagable |
|---|---|---|---|---|
| `FERIADO_LEGAL` | aniversario legal (+ progresivo) | 2 períodos | hábil | sí |
| `ADMINISTRATIVO` | año calendario | fin de año calendario | hábil | no |
| `RENDIMIENTO` | manual | días fijos | hábil | no |

`RENDIMIENTO` **no se siembra**. Se crea en vivo durante la demo, desde la pantalla de ABM,
para demostrar que agregar un tipo es crear un registro y no escribir código.

## 4. Reglas de cálculo

Todas en aritmética decimal de precisión fija.

**Feriado legal.** 15 días hábiles por aniversario de la fecha de ingreso.

**Progresivo.** Si `experiencia_total_meses >= 120` y `antiguedad_actual_meses >= 36`,
entonces `dias_extra = floor(años_con_empleador_actual / 3)`. El umbral (120), la
antigüedad mínima (36) y la cadencia (3) son parámetros de la política, no constantes del
código.

Caso de referencia: 9 años con el empleador actual → 3 días extra → 18 días totales.

**Proporcional (finiquito).** Sobre el período en curso desde el último aniversario:

```
meses_completos × 1,25 + (días_restantes / 30) × 1,25
```

Caso de referencia: `8 × 1,25 + (24/30) × 1,25 = 11,00`.

Al total se le suma el disponible de los tipos marcados como pagables. Es una consulta de
solo lectura: no escribe movimientos.

**Días hábiles.** Se descuentan domingos, sábados (el sábado es día inhábil por regla) y los
feriados legales vigentes del calendario del país.

**Consumo.** FIFO por `vence_el` ascendente, y a igualdad de fecha, por `prioridad_consumo`
del tipo. Una solicitud puede repartirse entre varias bolsas, y cada tramo genera su propio
movimiento `CONSUMPTION`.

**Vencimiento.** Al llegar la fecha, un movimiento `EXPIRATION` por el remanente exacto de
la bolsa.

## 5. Modelo de datos

Sigue el diagrama de referencia. Todas las tablas llevan `empresa_id` y todos los
repositorios filtran por él sin excepción: la multiempresa queda demostrada como estructura,
aunque se siembre una sola empresa.

| Tabla | Campos relevantes |
|---|---|
| `empresa` | `id`, `razon_social` |
| `colaborador` | `id`, `empresa_id`, `nombre`, `email`, `rol`, `fecha_ingreso`, `fecha_termino` (nullable), `meses_experiencia_previa`, `jornada` |
| `tipo_de_vacacion` | `id`, `empresa_id`, `codigo`, `politica_devengo`, `politica_vencimiento`, `parametros` (jsonb), `prioridad_consumo`, `unidad_habil`, `pagable_en_finiquito`, `vigente_desde` |
| `otorgamiento` | `id`, `colaborador_id`, `tipo_id`, `periodo_desde`, `periodo_hasta`, `dias_otorgados DECIMAL(6,2)`, `devengado_el`, `vence_el`, `origen` |
| `movimiento` | `id`, `otorgamiento_id`, `solicitud_id` (nullable), `cantidad DECIMAL(6,2)` con signo, `clase`, `fecha_efectiva`, `fecha_registro`, `actor_id`, `motivo`, `clave_idempotencia` UNIQUE, `reversa_de` (nullable) |
| `solicitud_de_vacaciones` | `id`, `colaborador_id`, `tipo_id`, `desde`, `hasta`, `dias_habiles`, `estado`, `aprobador_id`, `decidido_el` |
| `calendario_laboral` | `id`, `fecha`, `ambito`, `tipo` |

Clases de movimiento: `ACCRUAL`, `CONSUMPTION`, `EXPIRATION`, `ADJUSTMENT`, `REVERSAL`,
`SETTLEMENT_PAYOUT`, `OPENING_BALANCE`.

### Decisiones de persistencia que sostienen la tesis

**La inmutabilidad la impone Postgres, no Go.** El rol de aplicación recibe
`GRANT INSERT, SELECT ON movimiento` y nada más. Un `UPDATE` sobre el ledger falla aunque
alguien lo escriba por error. Cuesta una línea de SQL en la migración y es demostrable en
vivo desde `psql`.

**`vence_el` se persiste, no se recalcula.** La política calcula la fecha en el momento del
otorgamiento y se guarda. Si la política cambia después, el saldo pasado debe seguir siendo
explicable con las reglas que estaban vigentes cuando se otorgó.

**Bitemporalidad.** Cada movimiento guarda `fecha_efectiva` (cuándo ocurrió) y
`fecha_registro` (cuándo lo supimos). Eso permite responder "¿qué sabíamos el 3 de marzo?" y
no solo "¿qué es cierto hoy?".

**Idempotencia.** `clave_idempotencia` es UNIQUE en base de datos, con el formato
`{colaborador}:{tipo}:{periodo}:{clase}`. Reejecutar el job de devengo no duplica
movimientos; el `INSERT` simplemente colisiona y se ignora.

## 6. API

Todos los endpoints reciben el header `X-Actor-Id`, que el FE llena desde el selector de
usuario. El rol del actor determina qué campos se devuelven.

```
GET  /api/colaboradores                          RRHH: lista con disponible
GET  /api/colaboradores/{id}/saldo               desglose por tipo + próximos a vencer
GET  /api/colaboradores/{id}/movimientos         ledger completo; ?hasta= para saldo a fecha pasada
GET  /api/colaboradores/{id}/finiquito?fecha=    proporcional, solo lectura
GET  /api/solicitudes                            propias si colaborador, todas si RRHH
POST /api/solicitudes                            crear (calcula días hábiles y valida saldo)
POST /api/solicitudes/{id}/aprobar               RRHH: consumo FIFO en una transacción
POST /api/solicitudes/{id}/rechazar
GET  /api/tipos-vacacion
POST /api/tipos-vacacion                         ABM
PUT  /api/tipos-vacacion/{id}
POST /api/otorgamientos                          RRHH: carga manual, motivo obligatorio
POST /api/jobs/devengo?fecha=YYYY-MM-DD          idempotente, con fecha objetivo
POST /api/jobs/vencimiento?fecha=YYYY-MM-DD      idempotente, con fecha objetivo
GET  /api/calendario?desde=&hasta=               feriados, para el preview del FE
```

**Visibilidad por rol.** `GET /saldo` devuelve `devengado_no_otorgado` y el proporcional
**solo si el actor tiene rol RRHH**. Al colaborador se le muestra únicamente su disponible.
En la demo esto se ve al cambiar de usuario con el mismo colaborador en pantalla.

**Los jobs no tienen UI.** Se exponen como endpoints, se documentan en el README con su
`curl` y se explican verbalmente. Son idempotentes y aceptan fecha objetivo, que es
exactamente lo que permite recuperar días no procesados.

**Aprobación transaccional.** `POST /solicitudes/{id}/aprobar` corre dentro de una única
transacción con `SELECT ... FOR UPDATE` sobre el colaborador, lo que evita el doble gasto en
solicitudes concurrentes. Dentro de esa transacción: se ordenan las bolsas, se emite un
`CONSUMPTION` por cada una, y se cambia el estado de la solicitud.

**Reserva de días pendientes.** El disponible que se muestra para solicitar descuenta los
días de las solicitudes en estado `PENDIENTE`. No se escriben movimientos al crear una
solicitud — el ledger solo registra hechos consumados — pero el colaborador no puede
comprometer dos veces los mismos días.

## 7. Frontend

Header global con el selector de usuario (`Ver como: …`), persistido en cookie. El rol
decide qué navegación se muestra.

**Vista colaborador**
- Cards de disponible por tipo de vacación.
- Bloque "próximos a vencer" con fecha y cantidad.
- Formulario de solicitud: tipo + rango de fechas, con preview de días hábiles calculado por
  el API antes de enviar.
- Lista de mis solicitudes con su estado.
- Mi historial de movimientos.

**Vista RRHH**
- Tabla de colaboradores con su disponible.
- Detalle de colaborador: saldo, ledger completo, cálculo de finiquito, y acción de
  otorgamiento manual.
- Bandeja de solicitudes pendientes, con aprobar / rechazar.
- ABM de tipos de vacación.

## 8. Datos semilla

Tres personas. La historia del colaborador antiguo se genera **corriendo el motor de devengo
aniversario por aniversario**, no insertando saldos a mano. Es la diferencia entre un ledger
que demuestra el sistema y uno que solo lo ilustra.

**Ana Fuentes** — colaboradora, ingreso `2024-03-01`, sin experiencia previa. Caso simple:
dos otorgamientos de 15 días, sin progresivas.

**Carlos Rojas** — colaborador, ingreso `2017-04-15`, con 24 meses de experiencia previa
acreditada. Su ledger cuenta la historia completa:

| Aniversario | Años con el empleador | Experiencia total | Progresivo | Días otorgados |
|---|---|---|---|---|
| 2018 – 2024 | 1 – 7 | < 120 meses | — | 15,00 |
| 2025-04-15 | 8 | 96 + 24 = 120 ✓ | `floor(8/3)` = 2 | **17,00** |
| 2026-04-15 | 9 | 108 + 24 = 132 ✓ | `floor(9/3)` = 3 | **18,00** |

El salto de 15 a 17 y luego a 18 queda escrito en el ledger, con su fecha y su clave de
idempotencia. Eso es lo que se muestra al explicar el requisito de feriado progresivo.

Además, el seeder corre el job de vencimiento en cada corte. Con el tope de dos períodos,
las bolsas otorgadas hasta 2024 ya vencieron a la fecha de la demo y aparecen con su
`EXPIRATION` por el remanente exacto; quedan vivas la de 2025 (17,00) y la de 2026 (18,00).
El seeder siembra también dos o tres solicitudes aprobadas cuyo consumo se repartió FIFO
entre bolsas, de modo que los `EXPIRATION` sean por remanentes parciales y no por bolsas
intactas.

**Marta Silva** — RRHH, ingreso `2022-01-10`.

Tipos sembrados: `FERIADO_LEGAL` y `ADMINISTRATIVO`. Calendario laboral con los feriados
legales de Chile 2017–2027 (necesario hacia atrás para que el conteo histórico sea real).

## 9. Testing

Tests de dominio en Go, sin base de datos ni servidor. Cobertura dirigida a las reglas, no a
la infraestructura:

- Los dos casos de referencia del documento, con nombre explícito: 9 años → 18 días, y
  `8 × 1,25 + (24/30) × 1,25 = 11,00`.
- Progresivo en los bordes: justo bajo y justo sobre los umbrales de 120 y 36 meses.
- FIFO repartiendo una solicitud entre tres bolsas con distinta fecha de vencimiento.
- Días hábiles sobre un rango que contiene sábado, domingo y feriado.
- Idempotencia: correr el devengo dos veces para la misma fecha produce un solo movimiento.
- Vencimiento por remanente exacto tras un consumo parcial.

Sin tests de frontend.

## 10. Desviaciones respecto de los diagramas de referencia

Se documentan explícitamente porque son decisiones, no omisiones.

**`SALDO_CONSOLIDADO` no existe como tabla.** El diagrama lo incluye como proyección
derivada; en el MVP el saldo se calcula sumando el ledger en cada consulta. Con datos de
demo es instantáneo, y hace el argumento central más fuerte: el saldo literalmente no existe
como número editable en ninguna parte. La forma del DTO que devuelve el API es la misma
(`disponible`, `devengado_no_otorgado`, `por_vencer`), de modo que introducir el snapshot en
producción no cambia el contrato. El README explica el trade-off y por qué en producción sí
corresponde snapshot + job de reconciliación.

**El bus de eventos queda fuera.** El diagrama de secuencia publica `VacacionesAprobadas` vía
outbox hacia Nómina y Data Warehouse. Fuera del alcance de una demo de un solo servicio.

**Aprueba RRHH, no jefatura.** El diagrama muestra a la jefatura como aprobador. El MVP no
modela jerarquía; el rol RRHH aprueba. La máquina de estados y el campo `aprobador_id` son
idénticos, así que agregar el rol es cambiar quién puede llamar al endpoint.

**`movimiento` no lleva `colaborador_id` propio.** Se llega al colaborador por join a través
de `otorgamiento`, como en el diagrama. Es lo correcto en términos de normalización; en
producción con volumen se denormalizaría para poder particionar e indexar por colaborador.

**Se agrega `reversa_de` a `movimiento`.** El diagrama no lo incluye, pero la clase `REVERSAL`
sí existe en la propuesta. Sin ese campo, un reverso no puede señalar qué movimiento
reversa, y la trazabilidad se pierde.

## 11. Fuera de alcance

Declarado en el README:

- Snapshots de saldo y job de reconciliación.
- Multiempresa operativa (existe el campo y el filtro; no hay UI ni segunda empresa).
- Rol de jefatura y aprobaciones de más de un nivel.
- Notificaciones y avisos de vencimiento (60 y 15 días antes).
- Recálculo retroactivo ante corrección de la fecha de ingreso. La primitiva existe: la
  clase `REVERSAL` y el campo `reversa_de` están implementados y el ajuste manual los usa.
  Lo que no está es el proceso automático que reversa y reejecuta el motor.
- Monto en dinero del finiquito, licencias médicas, contratos por obra o faena, jornadas
  parciales con reglas especiales.
- Versionado de políticas con vigencia por fecha. Existe `vigente_desde` en el tipo, pero el
  motor no resuelve la versión aplicable según la fecha del cálculo.

## 12. Cómo se corre

```
make db-up      # docker compose up -d postgres
make migrate    # aplica las migraciones SQL
make seed       # siembra corriendo el motor sobre la historia
make api        # go run ./cmd/api        → :8080
make web        # npm run dev en web/     → :3000
make test       # go test ./internal/domain/...
```

El README debe explicar la solución de forma autónoma y genérica: el problema que resuelve,
la decisión del ledger y sus consecuencias, el modelo de datos, las reglas implementadas con
sus fórmulas, el mapa de endpoints incluidos los jobs con su `curl`, un guion de demo paso a
paso, y las limitaciones asumidas.
