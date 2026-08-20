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
