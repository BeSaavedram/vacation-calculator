package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/BeSaavedram/vacation-calculator/internal/app"
	"github.com/BeSaavedram/vacation-calculator/internal/domain"
)

func (s *Servidor) listarTipos(w http.ResponseWriter, r *http.Request) {
	tipos, err := s.svc.ListarTipos(r.Context())
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tipos == nil {
		tipos = []domain.TipoDeVacacion{}
	}
	responderJSON(w, http.StatusOK, tipos)
}

// cuerpoTipo es la forma en que el frontend describe un tipo de vacación.
type cuerpoTipo struct {
	Codigo              string            `json:"codigo"`
	Nombre              string            `json:"nombre"`
	PoliticaDevengo     string            `json:"politica_devengo"`
	PoliticaVencimiento string            `json:"politica_vencimiento"`
	Parametros          domain.Parametros `json:"parametros"`
	PrioridadConsumo    int               `json:"prioridad_consumo"`
	UnidadHabil         bool              `json:"unidad_habil"`
	PagableEnFiniquito  bool              `json:"pagable_en_finiquito"`
}

func (c cuerpoTipo) aDominio() domain.TipoDeVacacion {
	return domain.TipoDeVacacion{
		Codigo:              c.Codigo,
		Nombre:              c.Nombre,
		PoliticaDevengo:     domain.CodigoDevengo(c.PoliticaDevengo),
		PoliticaVencimiento: domain.CodigoVencimiento(c.PoliticaVencimiento),
		Parametros:          c.Parametros,
		PrioridadConsumo:    c.PrioridadConsumo,
		UnidadHabil:         c.UnidadHabil,
		PagableEnFiniquito:  c.PagableEnFiniquito,
	}
}

// crearTipo es el Requisito 7 completo: crear "días por rendimiento" es esta
// llamada, sin desarrollo ni despliegue.
func (s *Servidor) crearTipo(w http.ResponseWriter, r *http.Request) {
	var cuerpo cuerpoTipo
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	tipo, err := s.svc.CrearTipo(r.Context(), cuerpo.aDominio())
	if err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusCreated, tipo)
}

func (s *Servidor) actualizarTipo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		responderError(w, http.StatusBadRequest, "id inválido")
		return
	}

	var cuerpo cuerpoTipo
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	tipo := cuerpo.aDominio()
	tipo.ID = id
	if err := s.svc.ActualizarTipo(r.Context(), tipo); err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, tipo)
}

type cuerpoOtorgamiento struct {
	ColaboradorID string `json:"colaborador_id"`
	TipoID        string `json:"tipo_id"`
	Dias          string `json:"dias"`
	Motivo        string `json:"motivo"`
}

// otorgarManual carga un saldo especial. El motivo es obligatorio y queda
// escrito en el ledger junto al actor que lo cargó.
func (s *Servidor) otorgarManual(w http.ResponseWriter, r *http.Request) {
	actor := actorDe(r.Context())

	var cuerpo cuerpoOtorgamiento
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
		responderError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	colaboradorID, err := uuid.Parse(cuerpo.ColaboradorID)
	if err != nil {
		responderError(w, http.StatusBadRequest, "colaborador_id inválido")
		return
	}
	tipoID, err := uuid.Parse(cuerpo.TipoID)
	if err != nil {
		responderError(w, http.StatusBadRequest, "tipo_id inválido")
		return
	}
	dias, err := decimal.NewFromString(cuerpo.Dias)
	if err != nil {
		responderError(w, http.StatusBadRequest, "dias debe ser un número decimal, por ejemplo \"2.5\"")
		return
	}

	if err := s.svc.OtorgarManual(r.Context(), colaboradorID, tipoID, dias, cuerpo.Motivo, actor.ID); err != nil {
		responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	responderJSON(w, http.StatusCreated, map[string]string{"estado": "otorgado"})
}

func (s *Servidor) correrDevengo(w http.ResponseWriter, r *http.Request) {
	s.correrJob(w, r, s.svc.CorrerDevengo)
}

func (s *Servidor) correrVencimiento(w http.ResponseWriter, r *http.Request) {
	s.correrJob(w, r, s.svc.CorrerVencimiento)
}

type funcionJob func(ctx context.Context, fecha time.Time, actorID uuid.UUID) (app.ResultadoJob, error)

func (s *Servidor) correrJob(w http.ResponseWriter, r *http.Request, job funcionJob) {
	actor := actorDe(r.Context())

	fecha, err := leerFecha(r, "fecha", s.svc.Hoy())
	if err != nil {
		responderError(w, http.StatusBadRequest, "fecha debe tener formato YYYY-MM-DD")
		return
	}

	resultado, err := job(r.Context(), fecha, actor.ID)
	if err != nil {
		responderError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responderJSON(w, http.StatusOK, resultado)
}
