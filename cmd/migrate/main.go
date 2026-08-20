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
