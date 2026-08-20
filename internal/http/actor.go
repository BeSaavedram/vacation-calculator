package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/BeSaavedram/vacation-calculator/internal/domain"
	"github.com/BeSaavedram/vacation-calculator/internal/store"
)

type claveContexto string

const claveActor claveContexto = "actor"

// conActor resuelve quién está haciendo la llamada a partir del header
// X-Actor-Id, que el frontend llena desde el selector de usuario.
//
// Esto NO es autenticación: es la demo de un modelo de permisos. En producción
// el actor saldría de un token firmado. Lo que sí es real es el uso que se le
// da: el rol del actor decide qué datos se devuelven.
func (s *Servidor) conActor(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		crudo := r.Header.Get("X-Actor-Id")
		if crudo == "" {
			responderError(w, http.StatusUnauthorized, "falta el header X-Actor-Id")
			return
		}
		id, err := uuid.Parse(crudo)
		if err != nil {
			responderError(w, http.StatusBadRequest, "X-Actor-Id no es un uuid válido")
			return
		}

		actor, err := store.ColaboradorPorIDSinEmpresa(r.Context(), s.pool, id)
		if err != nil {
			responderError(w, http.StatusUnauthorized, "actor desconocido")
			return
		}

		ctx := context.WithValue(r.Context(), claveActor, actor)
		siguiente(w, r.WithContext(ctx))
	}
}

// actorDe recupera el actor que el middleware dejó en el contexto.
func actorDe(ctx context.Context) domain.Colaborador {
	actor, _ := ctx.Value(claveActor).(domain.Colaborador)
	return actor
}

// soloRRHH corta la llamada si el actor no tiene rol de RRHH.
func soloRRHH(siguiente http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !actorDe(r.Context()).EsRRHH() {
			responderError(w, http.StatusForbidden, "esta acción requiere rol de RRHH")
			return
		}
		siguiente(w, r)
	}
}
