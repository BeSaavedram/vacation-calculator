# Gestión de Vacaciones

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

> Si el API estaba corriendo, reinícialo después de un `demo-reset`. Resuelve el identificador de la empresa una sola vez al arrancar, así que un proceso viejo queda apuntando a datos que ya no existen y responde listas vacías en vez de un error — que es más confuso que fallar.

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
