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
