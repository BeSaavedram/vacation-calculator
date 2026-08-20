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
