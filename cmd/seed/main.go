package main

import (
	"context"
	"log"
	"os"
	"sort"
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
