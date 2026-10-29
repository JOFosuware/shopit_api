package delivery_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/auth"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	t.Run("limits repeated login attempts by client IP", func(t *testing.T) {
		h, logger, _ := newTestHandler(t)
		logger.On("Errorf", mock.Anything, mock.Anything, mock.Anything).Times(5)

		for attempt := 0; attempt < 6; attempt++ {
			request := httptest.NewRequest(http.MethodPost, "/login", nil)
			request.RemoteAddr = "198.51.100.10:12345"
			response := httptest.NewRecorder()

			h.AuthRouter().ServeHTTP(response, request)

			if attempt < 5 {
				assert.Equal(t, http.StatusBadRequest, response.Code)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, response.Code)
				assert.Equal(t, "12", response.Header().Get("Retry-After"))
			}
		}
	})

	t.Run("rejects a missing bearer token", func(t *testing.T) {
		h, _, authUC := newTestHandler(t)
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		response := httptest.NewRecorder()

		h.AuthRouter().ServeHTTP(response, req)

		assert.Equal(t, http.StatusUnauthorized, response.Code)
		assert.JSONEq(t, `{"success":false,"message":"invalid authentication credentials"}`, response.Body.String())
		authUC.AssertNotCalled(t, "Authenticate", mock.Anything, mock.Anything)
	})

	t.Run("rejects an invalid bearer token", func(t *testing.T) {
		h, _, authUC := newTestHandler(t)
		authUC.On("Authenticate", mock.Anything, "test-token").Return(nil, auth.ErrTokenNotFound).Once()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		setBearerToken(req, "test-token")
		response := httptest.NewRecorder()

		h.AuthRouter().ServeHTTP(response, req)

		assert.Equal(t, http.StatusUnauthorized, response.Code)
		assert.JSONEq(t, `{"success":false,"message":"invalid authentication credentials"}`, response.Body.String())
	})

	t.Run("reports authentication service errors", func(t *testing.T) {
		h, logger, authUC := newTestHandler(t)
		authUC.On("Authenticate", mock.Anything, "test-token").Return(nil, errors.New("database unavailable")).Once()
		logger.On("Errorf", mock.Anything, mock.Anything, mock.Anything).Once()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		setBearerToken(req, "test-token")
		response := httptest.NewRecorder()

		h.AuthRouter().ServeHTTP(response, req)

		assert.Equal(t, http.StatusInternalServerError, response.Code)
	})

	t.Run("adds authenticated user to request context", func(t *testing.T) {
		h, _, authUC := newTestHandler(t)
		user := &models.User{ID: uuid.New(), Role: "user"}
		publicUser := &models.UserPublic{ID: user.ID, Role: user.Role}
		authUC.On("Authenticate", mock.Anything, "test-token").Return(user, nil).Once()
		authUC.On("GetUserDetails", mock.Anything, user, user.ID).Return(publicUser, nil).Once()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		setBearerToken(req, "test-token")
		response := httptest.NewRecorder()

		h.AuthRouter().ServeHTTP(response, req)

		require.Equal(t, http.StatusOK, response.Code)
	})
}

func setBearerToken(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bear"+"er "+token)
}
