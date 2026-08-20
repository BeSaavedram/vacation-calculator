package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

// Usuario es una entrada del selector del frontend.
type Usuario struct {
	ID     uuid.UUID `json:"id"`
	Nombre string    `json:"nombre"`
	Rol    string    `json:"rol"`
	Email  string    `json:"email"`
}

// listarUsuarios alimenta el selector "Ver como…". No requiere actor: es el
// punto de entrada de la demo.
func (s *Servidor) listarUsuarios(w http.ResponseWriter, r *http.Request) {
	colaboradores, err := store.ListarColaboradores(r.Context(), s.pool, s.svc.EmpresaID)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}

	usuarios := make([]Usuario, 0, len(colaboradores))
	for _, c := range colaboradores {
		usuarios = append(usuarios, Usuario{ID: c.ID, Nombre: c.Nombre, Rol: string(c.Rol), Email: c.Email})
	}
	responderJSON(w, http.StatusOK, usuarios)
}

// ColaboradorConSaldo es una fila de la tabla de RRHH.
type ColaboradorConSaldo struct {
	ID           uuid.UUID `json:"id"`
	Nombre       string    `json:"nombre"`
	Email        string    `json:"email"`
	Rol          string    `json:"rol"`
	FechaIngreso time.Time `json:"fecha_ingreso"`
	Antiguedad   int       `json:"anios_antiguedad"`
	Disponible   string    `json:"disponible"`
}

func (s *Servidor) listarColaboradores(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hoy := s.svc.Hoy()

	colaboradores, err := store.ListarColaboradores(ctx, s.pool, s.svc.EmpresaID)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}

	filas := make([]ColaboradorConSaldo, 0, len(colaboradores))
	for _, c := range colaboradores {
		saldo, err := s.svc.Saldo(ctx, c.ID, true)
		if err != nil {
			responderError(w, http.StatusInternalServerError, err.Error())
			return
		}
		filas = append(filas, ColaboradorConSaldo{
			ID:           c.ID,
			Nombre:       c.Nombre,
			Email:        c.Email,
			Rol:          string(c.Rol),
			FechaIngreso: c.FechaIngreso,
			Antiguedad:   c.AniosConEmpleadorAl(hoy),
			Disponible:   saldo.Total.String(),
		})
	}
	responderJSON(w, http.StatusOK, filas)
}

// verSaldo devuelve el saldo proyectado. Un colaborador solo puede ver el suyo;
// RRHH puede ver el de cualquiera, y además recibe el devengado no otorgado.
func (s *Servidor) verSaldo(w http.ResponseWriter, r *http.Request) {
	actor := actorDe(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if !actor.EsRRHH() && actor.ID != id {
		responderError(w, http.StatusForbidden, "solo puedes consultar tu propio saldo")
		return
	}

	saldo, err := s.svc.Saldo(r.Context(), id, actor.EsRRHH())
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, saldo)
}

// verHistorial devuelve el ledger. Con ?hasta=YYYY-MM-DD reconstruye el saldo a
// una fecha pasada: es la demostración directa del Requisito 5.
func (s *Servidor) verHistorial(w http.ResponseWriter, r *http.Request) {
	actor := actorDe(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if !actor.EsRRHH() && actor.ID != id {
		responderError(w, http.StatusForbidden, "solo puedes consultar tu propio historial")
		return
	}

	var hasta *time.Time
	if crudo := r.URL.Query().Get("hasta"); crudo != "" {
		f, err := time.Parse("2006-01-02", crudo)
		if err != nil {
			responderError(w, http.StatusBadRequest, "hasta debe tener formato YYYY-MM-DD")
			return
		}
		hasta = &f
	}

	movimientos, err := s.svc.Historial(r.Context(), id, hasta)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if movimientos == nil {
		movimientos = []store.MovimientoConContexto{}
	}
	responderJSON(w, http.StatusOK, movimientos)
}

func (s *Servidor) verFiniquito(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}
	fecha, err := leerFecha(r, "fecha", s.svc.Hoy())
	if err != nil {
		responderError(w, http.StatusBadRequest, "fecha debe tener formato YYYY-MM-DD")
		return
	}

	finiquito, err := s.svc.Finiquito(r.Context(), id, fecha)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, finiquito)
}
