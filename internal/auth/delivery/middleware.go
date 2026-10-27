package delivery

import (
	"context"
	"errors"
	"net/http"

	"github.com/jofosuware/go/shopit/internal/auth"
	"github.com/jofosuware/go/shopit/internal/models"
)

func (h *AuthHandlers) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plainTextToken := bearerToken(r)
		if plainTextToken == "" {
			h.writeError(w, http.StatusUnauthorized, "invalid authentication credentials")
			return
		}

		user, err := h.authUC.Authenticate(r.Context(), plainTextToken)
		if err != nil {
			if errors.Is(err, auth.ErrTokenNotFound) {
				h.writeError(w, http.StatusUnauthorized, "invalid authentication credentials")
				return
			}
			h.writeInternalError(w, "auth middleware: resolving bearer token", err)
			return
		}
		if user == nil {
			h.writeInternalError(w, "auth middleware: use case returned no user", errors.New("authentication use case returned no user"))
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authenticatedUser(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	return user, ok && user != nil
}
