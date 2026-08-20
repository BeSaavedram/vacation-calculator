package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BeSaavedram/vacation-calculator/internal/app"
	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

// Servidor expone los casos de uso por HTTP.
type Servidor struct {
	svc  *app.Servicio
	pool *pgxpool.Pool
}

// NuevoServidor arma el router con todas las rutas.
func NuevoServidor(svc *app.Servicio, pool *pgxpool.Pool) http.Handler {
	s := &Servidor{svc: svc, pool: pool}
	mux := http.NewServeMux()

	// Sin actor: alimenta el selector de usuario del frontend.
	mux.HandleFunc("GET /api/usuarios", s.listarUsuarios)
	mux.HandleFunc("GET /api/salud", func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, http.StatusOK, map[string]string{"estado": "ok"})
	})

	// Colaboradores y saldo
	mux.HandleFunc("GET /api/colaboradores", s.conActor(soloRRHH(s.listarColaboradores)))
	mux.HandleFunc("GET /api/colaboradores/{id}/saldo", s.conActor(s.verSaldo))
	mux.HandleFunc("GET /api/colaboradores/{id}/movimientos", s.conActor(s.verHistorial))
	mux.HandleFunc("GET /api/colaboradores/{id}/finiquito", s.conActor(soloRRHH(s.verFiniquito)))

	// Solicitudes
	mux.HandleFunc("GET /api/solicitudes", s.conActor(s.listarSolicitudes))
	mux.HandleFunc("POST /api/solicitudes", s.conActor(s.crearSolicitud))
	mux.HandleFunc("GET /api/solicitudes/preview", s.conActor(s.previewSolicitud))
	mux.HandleFunc("POST /api/solicitudes/{id}/aprobar", s.conActor(soloRRHH(s.aprobarSolicitud)))
	mux.HandleFunc("POST /api/solicitudes/{id}/rechazar", s.conActor(soloRRHH(s.rechazarSolicitud)))

	// Configuración y carga manual
	mux.HandleFunc("GET /api/tipos-vacacion", s.conActor(s.listarTipos))
	mux.HandleFunc("POST /api/tipos-vacacion", s.conActor(soloRRHH(s.crearTipo)))
	mux.HandleFunc("PUT /api/tipos-vacacion/{id}", s.conActor(soloRRHH(s.actualizarTipo)))
	mux.HandleFunc("POST /api/otorgamientos", s.conActor(soloRRHH(s.otorgarManual)))

	// Jobs. Sin interfaz: se explican y se corren con curl.
	mux.HandleFunc("POST /api/jobs/devengo", s.conActor(soloRRHH(s.correrDevengo)))
	mux.HandleFunc("POST /api/jobs/vencimiento", s.conActor(soloRRHH(s.correrVencimiento)))

	return conCORS(mux)
}

// conCORS permite que el frontend de desarrollo en :3000 llame al API en :8080.
func conCORS(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Actor-Id")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		siguiente.ServeHTTP(w, r)
	})
}

func responderJSON(w http.ResponseWriter, codigo int, cuerpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	_ = json.NewEncoder(w).Encode(cuerpo)
}

func responderError(w http.ResponseWriter, codigo int, mensaje string) {
	responderJSON(w, codigo, map[string]string{"error": mensaje})
}

// leerFecha lee un parámetro de query con formato YYYY-MM-DD. Si viene vacío,
// devuelve porDefecto.
func leerFecha(r *http.Request, nombre string, porDefecto time.Time) (time.Time, error) {
	crudo := r.URL.Query().Get(nombre)
	if crudo == "" {
		return porDefecto, nil
	}
	f, err := time.Parse("2006-01-02", crudo)
	if err != nil {
		return time.Time{}, err
	}
	return domain.SoloFecha(f), nil
}
