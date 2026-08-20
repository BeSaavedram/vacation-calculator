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
