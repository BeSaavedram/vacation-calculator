package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
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
	base := os.Getenv("PORT")
	if base == "" {
		base = "8080"
	}
	puerto := portLibre(base)

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

// portLibre devuelve el primer puerto disponible a partir de base.
func portLibre(base string) string {
	n, err := strconv.Atoi(base)
	if err != nil {
		return base
	}
	for p := n; p < n+10; p++ {
		addr := fmt.Sprintf(":%d", p)
		l, err := net.Listen("tcp", addr)
		if err == nil {
			l.Close()
			if p != n {
				log.Printf("puerto %d ocupado, usando %d", n, p)
			}
			return strconv.Itoa(p)
		}
	}
	log.Fatalf("no hay puerto disponible en el rango %d-%d", n, n+9)
	return ""
}

// empresaUnica resuelve el id de la empresa. Este MVP opera con una sola, pero
// el id viaja explícito en cada consulta: la multiempresa está en la estructura
// aunque no esté en la interfaz.
func empresaUnica(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM empresa ORDER BY razon_social LIMIT 1`).Scan(&id)
	return id, err
}
