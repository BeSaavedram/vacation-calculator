package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/app"
	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

func (s *Servidor) listarSolicitudes(w http.ResponseWriter, r *http.Request) {
	solicitudes, err := s.svc.ListarSolicitudes(r.Context(), actorDe(r.Context()))
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if solicitudes == nil {
		solicitudes = []store.Solicitud{}
	}
	responderJSON(w, http.StatusOK, solicitudes)
}

// previewSolicitud cuenta los días hábiles de un rango antes de confirmar, para
// que el colaborador vea el descuento real y no el número de días corridos.
func (s *Servidor) previewSolicitud(w http.ResponseWriter, r *http.Request) {
	desde, err := leerFecha(r, "desde", time.Time{})
	if err != nil || desde.IsZero() {
		responderError(w, http.StatusBadRequest, "desde es obligatorio con formato YYYY-MM-DD")
		return
	}
	hasta, err := leerFecha(r, "hasta", time.Time{})
	if err != nil || hasta.IsZero() {
		responderError(w, http.StatusBadRequest, "hasta es obligatorio con formato YYYY-MM-DD")
		return
	}
	if hasta.Before(desde) {
		responderError(w, http.StatusBadRequest, "hasta no puede ser anterior a desde")
		return
	}

	preview, err := s.svc.PreviewDiasHabiles(r.Context(), desde, hasta)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, preview)
}

type cuerpoCrearSolicitud struct {
	TipoID string `json:"tipo_id"`
	Desde  string `json:"desde"`
	Hasta  string `json:"hasta"`
}

func (s *Servidor) crearSolicitud(w http.ResponseWriter, r *http.Request) {
	actor := actorDe(r.Context())

	var cuerpo cuerpoCrearSolicitud
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	tipoID, err := uuid.Parse(cuerpo.TipoID)
	if err != nil {
		responderError(w, http.StatusBadRequest, "tipo_id inválido")
		return
	}
	desde, err := time.Parse("2006-01-02", cuerpo.Desde)
	if err != nil {
		responderError(w, http.StatusBadRequest, "desde debe tener formato YYYY-MM-DD")
		return
	}
	hasta, err := time.Parse("2006-01-02", cuerpo.Hasta)
	if err != nil {
		responderError(w, http.StatusBadRequest, "hasta debe tener formato YYYY-MM-DD")
		return
	}

	solicitud, err := s.svc.CrearSolicitud(r.Context(), actor.ID, tipoID, desde, hasta)
	switch {
	case errors.Is(err, domain.ErrSaldoInsuficiente), errors.Is(err, app.ErrRangoInvalido):
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case err != nil:
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusCreated, solicitud)
}

func (s *Servidor) aprobarSolicitud(w http.ResponseWriter, r *http.Request) {
	s.decidirSolicitud(w, r, true)
}

func (s *Servidor) rechazarSolicitud(w http.ResponseWriter, r *http.Request) {
	s.decidirSolicitud(w, r, false)
}

func (s *Servidor) decidirSolicitud(w http.ResponseWriter, r *http.Request, aprobar bool) {
	actor := actorDe(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}

	if aprobar {
		err = s.svc.AprobarSolicitud(r.Context(), id, actor.ID)
	} else {
		err = s.svc.RechazarSolicitud(r.Context(), id, actor.ID)
	}

	switch {
	case errors.Is(err, app.ErrYaDecidida):
		responderError(w, http.StatusConflict, err.Error())
		return
	case errors.Is(err, domain.ErrSaldoInsuficiente):
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case err != nil:
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}

	solicitud, err := store.SolicitudPorID(r.Context(), s.pool, s.svc.EmpresaID, id)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, solicitud)
}
