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

No existe ninguna columna `saldo` en ninguna tabla. Cuando el sistema responde "tienes 35 días disponibles", ese número se acaba de calcular sumando el historial. Un número que no existe no puede quedar desactualizado.

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

**Aniversarios de fin de mes.** El aniversario es el día de ingreso recortado al último día del mes cuando ese mes es más corto. Quien ingresa un 29 de febrero cumple aniversario el 28 en años comunes y el 29 en bisiestos. Sin ese recorte, un ingreso del 29 de febrero no devenga nunca en años comunes.

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

La regla estructural que sostiene todo lo demás: **`internal/domain` no importa infraestructura**. Las reglas de cálculo se testean sin base de datos y sin servidor —61 tests que corren en menos de medio segundo— y se pueden leer completas sin atravesar capas.

**Stack:** Go 1.25 con el router de la biblioteca estándar, PostgreSQL 16, `pgx/v5`, `shopspring/decimal`, Next.js con TypeScript y Tailwind. Tres dependencias en el servidor, deliberadamente.

### Tres decisiones de persistencia

**La inmutabilidad la impone la base de datos.** El rol de aplicación recibe `GRANT SELECT, INSERT` sobre la tabla de movimientos y nada más. Un `UPDATE` sobre el ledger falla en el motor de base de datos, aunque alguien lo escriba por error en el código:

```bash
docker compose exec -T postgres psql -U vacaciones_app -d vacaciones -c "UPDATE movimiento SET cantidad = 0;"
```

```
ERROR:  permission denied for table movimiento
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
```

Luego, en dos terminales:

```bash
make api        # API en http://localhost:8080
```

```bash
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

El historial de Carlos **no está insertado a mano**: la semilla recorre su historia completa corriendo el mismo motor que corre en producción, fecha por fecha y en orden cronológico. El ledger resultante es el que el sistema habría producido si hubiera estado funcionando desde su primer día.

Por eso su historial muestra el efecto de las progresivas en dos escalones:

| Aniversario | Años con el empleador | Experiencia total | Días otorgados |
|---|---|---|---|
| 1º a 7º | 1 – 7 | bajo el umbral | 15,00 |
| 8º | 8 | alcanza los 120 meses | **17,00** |
| 9º | 9 | — | **18,00** |

Y como sus vacaciones se consumen en el punto cronológico correcto, las bolsas antiguas vencen por su **remanente parcial** y no intactas: los 10 días que tomó en febrero salen FIFO de la bolsa más próxima a vencer, que muere después con −5,00 en lugar de −15,00.

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

Los identificadores de la semilla cambian en cada `make seed`. Para obtener el de RRHH:

```bash
curl -s http://localhost:8080/api/usuarios
```

### Los procesos automáticos

El devengo y el vencimiento son jobs. No tienen interfaz: se exponen como endpoints y en producción los dispararía un scheduler.

```bash
curl -X POST -H "X-Actor-Id: $ID_RRHH" "http://localhost:8080/api/jobs/devengo?fecha=2026-04-15"
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
