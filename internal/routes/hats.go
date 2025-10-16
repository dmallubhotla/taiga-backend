package routes

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gitea.deepak.science/deepak/trygo/internal/models"
	"gitea.deepak.science/deepak/trygo/internal/tokens"
	"github.com/go-chi/chi/v5"
)

func newHatRouter(m models.Model) http.Handler {
	router := chi.NewRouter()
	router.Get("/{hatid}", getHatFunc(m))

	return router
}

func getHatFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		hatID64, err := strconv.ParseInt(chi.URLParam(r, "hatid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		hatID := int32(hatID64)

		hat, err := m.Hat(ctx, hatID, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(hat); err != nil {
			serverError(w, err)
		}

	}
}
