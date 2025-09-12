// Package delivery contains unit tests for the authentication HTTP handlers.
//
// These tests cover all success, error, and edge cases for each handler, using table-driven and subtest patterns. Test helpers are provided for DRY, isolated setup of mocks and handlers. All tests use testify for assertions and mocking, and ensure proper test isolation and maintainability.
//
// See handlers.go for handler implementation and GoDoc for handler documentation.
package delivery_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/auth/delivery"
	mockAuth "github.com/jofosuware/go/shopit/internal/auth/mocks"
	"github.com/jofosuware/go/shopit/internal/models"
	mockLogger "github.com/jofosuware/go/shopit/pkg/logger/mock"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// UserContextKey is the key used to store the user in the request context.
const UserContextKey = utils.UserContextKey

// newTestHandler is a test helper that returns a new AuthHandlers instance and its mocked dependencies.
// It ensures each test has isolated mocks and handler setup.
func newTestHandler(t *testing.T) (*delivery.AuthHandlers, *mockLogger.Logger, *mockAuth.AuthenticateUC) {
	logger := mockLogger.NewLogger(t)
	authUC := mockAuth.NewAuthenticateUC(t)
	h := delivery.NewAuthHandlers(logger, authUC)
	return h, logger, authUC
}

// TestRegister tests the Register handler for user registration, covering success, missing fields, multipart parsing errors, and use case errors.
func TestRegister(t *testing.T) {
	h, logger, authUC := newTestHandler(t)

	tests := []struct {
		name       string
		formData   url.Values
		avatar     string
		mockReturn interface{}
		mockError  error
		wantCode   int
	}{
		{
			name: "Successful registration",
			formData: url.Values{
				"name":     {"John Doe"},
				"email":    {"user@gmail.com"},
				"password": {"veryStrongPassword"},
				"avatar":   {"someImage.jpg"},
			},
			avatar:     "someImage.jpg",
			mockReturn: &models.UserResponse{},
			mockError:  nil,
			wantCode:   http.StatusOK,
		},
		{
			name: "Missing required field",
			formData: url.Values{
				"email":    {"user@gmail.com"},
				"password": {"veryStrongPassword"},
				"avatar":   {"someImage.jpg"},
			},
			avatar:     "someImage.jpg",
			mockReturn: nil,
			mockError:  assert.AnError,
			wantCode:   http.StatusUnprocessableEntity,
		},
		{
			name: "ParseMultipartForm error",
			formData: url.Values{},
			avatar: "someImage.jpg",
			mockReturn: nil,
			mockError: nil,
			wantCode: http.StatusBadRequest,
		},
		{
			name: "authUC.Register error",
			formData: url.Values{
				"name":     {"John Doe"},
				"email":    {"user@gmail.com"},
				"password": {"veryStrongPassword"},
				"avatar":   {"someImage.jpg"},
			},
			avatar:     "someImage.jpg",
			mockReturn: nil,
			mockError:  assert.AnError,
			wantCode:   http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "ParseMultipartForm error" {
				// Simulate ParseMultipartForm error by passing a nil body
				req := httptest.NewRequest(http.MethodPost, "/register", nil)
				req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
				rr := httptest.NewRecorder()
				logger.On("Errorf", mock.Anything, mock.Anything).Once()
				h.Register(rr, req)
				assert.Equal(t, tt.wantCode, rr.Code)
				logger.AssertExpectations(t)
				return
			}
			frmData, ct, _ := utils.CreateMultipartForm(tt.formData)
			req := httptest.NewRequest(http.MethodPost, "/register", frmData)
			req.Header.Set("Content-Type", ct)
			rr := httptest.NewRecorder()

			u := models.User{
				Name:     tt.formData.Get("name"),
				Email:    tt.formData.Get("email"),
				Password: tt.formData.Get("password"),
				Role:     "user",
			}

			if tt.name == "authUC.Register error" {
				authUC.On("Register", u, tt.avatar).Return(nil, tt.mockError).Once()
				logger.On("Errorf", mock.Anything, mock.Anything).Once()
				h.Register(rr, req)
				assert.Equal(t, tt.wantCode, rr.Code)
				authUC.AssertExpectations(t)
				logger.AssertExpectations(t)
				return
			}

			if tt.mockError == nil {
				authUC.On("Register", u, tt.avatar).Return(tt.mockReturn, nil).Once()
			} else {
				logger.On("Errorf", mock.Anything, mock.Anything).Once()
				authUC.On("Register", u, tt.avatar).Return(nil, tt.mockError).Maybe()
			}

			h.Register(rr, req)
			assert.Equal(t, tt.wantCode, rr.Code)
			authUC.AssertExpectations(t)
			logger.AssertExpectations(t)
		})
	}
}

// TestLogin tests the Login handler for user authentication, covering success, invalid credentials, malformed JSON, and validation errors.
func TestLogin(t *testing.T) {
	h, logger, authUC := newTestHandler(t)

	tests := []struct {
		name      string
		jsonData  []byte
		mockUser  models.User
		mockResp  interface{}
		mockError error
		wantCode  int
	}{
		{
			name:     "Successful login",
			jsonData: []byte(`{"email": "user@gmail.com", "password": "Science@1992"}`),
			mockUser: models.User{Email: "user@gmail.com", Password: "Science@1992"},
			mockResp: &models.UserResponse{},
			mockError: nil,
			wantCode: http.StatusOK,
		},
		{
			name:     "Invalid credentials",
			jsonData: []byte(`{"email": "user@gmail.com", "password": "wrongpass"}`),
			mockUser: models.User{Email: "user@gmail.com", Password: "wrongpass"},
			mockResp: nil,
			mockError: assert.AnError,
			wantCode: http.StatusBadRequest,
		},
		{
			name: "Malformed JSON",
			jsonData: []byte(`{"email": "user@gmail.com", "password": "Science@1992"`), // missing closing brace
			mockUser: models.User{},
			mockResp: nil,
			mockError: nil,
			wantCode: http.StatusBadRequest,
		},
		{
			name: "Validation error - missing email",
			jsonData: []byte(`{"email": "", "password": "Science@1992"}`),
			mockUser: models.User{Email: "", Password: "Science@1992"},
			mockResp: nil,
			mockError: nil,
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			name: "Validation error - short password",
			jsonData: []byte(`{"email": "user@gmail.com", "password": "short"}`),
			mockUser: models.User{Email: "user@gmail.com", Password: "short"},
			mockResp: nil,
			mockError: nil,
			wantCode: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Malformed JSON" {
				req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(tt.jsonData))
				rr := httptest.NewRecorder()
				logger.On("Errorf", mock.Anything, mock.Anything).Once()
				h.Login(rr, req)
				assert.Equal(t, tt.wantCode, rr.Code)
				logger.AssertExpectations(t)
				return
			}
			if tt.name == "Validation error - missing email" || tt.name == "Validation error - short password" {
				req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(tt.jsonData))
				rr := httptest.NewRecorder()
				logger.On("Errorf", mock.Anything, mock.Anything).Once()
				h.Login(rr, req)
				assert.Equal(t, tt.wantCode, rr.Code)
				logger.AssertExpectations(t)
				return
			}
			req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(tt.jsonData))
			rr := httptest.NewRecorder()
			if tt.mockError == nil {
				authUC.On("Login", tt.mockUser.Email, tt.mockUser.Password).Return(tt.mockResp, nil).Once()
			} else {
				logger.On("Errorf", mock.Anything, mock.Anything).Once()
				authUC.On("Login", tt.mockUser.Email, tt.mockUser.Password).Return(nil, tt.mockError).Once()
			}
			h.Login(rr, req)
			assert.Equal(t, tt.wantCode, rr.Code)
			authUC.AssertExpectations(t)
			logger.AssertExpectations(t)
		})
	}
}

// TestSendPasswordResetEmail tests the SendPasswordResetEmail handler, covering success, missing fields, multipart parsing errors, and use case errors.
func TestSendPasswordResetEmail(t *testing.T) {
	h, logger, authUC := newTestHandler(t)

	t.Run("Successful send password reset email", func(t *testing.T) {
		formData, ct, _ := utils.CreateMultipartForm(url.Values{"email": {"user@gmail.com"}})
		req, err := http.NewRequest(http.MethodPost, "/send-password-reset-email", formData)
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		authUC.On("SendPasswordResetEmail", "user@gmail.com", req).Return(&models.Response{}, nil).Once()
		req.Header.Set("Content-Type", ct)
		h.SendPasswordResetEmail(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		authUC.AssertExpectations(t)
	})

	t.Run("Missing email field", func(t *testing.T) {
		formData, ct, _ := utils.CreateMultipartForm(url.Values{})
		req, err := http.NewRequest(http.MethodPost, "/send-password-reset-email", formData)
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		req.Header.Set("Content-Type", ct)
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.SendPasswordResetEmail(rr, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("ParseMultipartForm error", func(t *testing.T) {
		// Simulate ParseMultipartForm error by passing a nil body
		req := httptest.NewRequest(http.MethodPost, "/send-password-reset-email", nil)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		rr := httptest.NewRecorder()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.SendPasswordResetEmail(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("authUC.SendPasswordResetEmail error", func(t *testing.T) {
		formData, ct, _ := utils.CreateMultipartForm(url.Values{"email": {"user@gmail.com"}})
		req, err := http.NewRequest(http.MethodPost, "/send-password-reset-email", formData)
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		req.Header.Set("Content-Type", ct)
		authUC.On("SendPasswordResetEmail", "user@gmail.com", req).Return(nil, assert.AnError).Once()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.SendPasswordResetEmail(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		authUC.AssertExpectations(t)
		logger.AssertExpectations(t)
	})
}

// TestResetPassword tests the ResetPassword handler for password reset functionality, covering success, mismatched passwords, multipart parsing errors, validation errors, and use case errors.
func TestResetPassword(t *testing.T) {
	h, logger, authUC := newTestHandler(t)

	formData := url.Values{}
	formData.Set("password", "verySecret")
	formData.Set("confirmPassword", "verySecret")
	body, contentType, err := utils.CreateMultipartForm(formData)
	require.NoError(t, err)

	t.Run("Successful reset password", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/reset-password", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", contentType)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		rr := httptest.NewRecorder()
		authUC.On("ResetPassword", "dummy-token", "verySecret").Return(&models.UserResponse{}, nil).Once()
		h.ResetPassword(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		authUC.AssertExpectations(t)
	})

	t.Run("Mismatched passwords", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("password", "verySecret")
		formData.Set("confirmPassword", "notMatch")
		body, contentType, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/reset-password", body)
		req.Header.Set("Content-Type", contentType)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		rr := httptest.NewRecorder()
		logger.On("Info", mock.Anything).Once()
		h.ResetPassword(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("ParseMultipartForm error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/reset-password", nil)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		rr := httptest.NewRecorder()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.ResetPassword(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("Validation error - missing password", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("confirmPassword", "verySecret")
		body, contentType, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/reset-password", body)
		req.Header.Set("Content-Type", contentType)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		rr := httptest.NewRecorder()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.ResetPassword(rr, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("Validation error - missing confirmPassword", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("password", "verySecret")
		body, contentType, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/reset-password", body)
		req.Header.Set("Content-Type", contentType)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		rr := httptest.NewRecorder()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.ResetPassword(rr, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("authUC.ResetPassword error", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("password", "verySecret")
		formData.Set("confirmPassword", "verySecret")
		body, contentType, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/reset-password", body)
		req.Header.Set("Content-Type", contentType)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rCtx))
		rr := httptest.NewRecorder()
		authUC.On("ResetPassword", "dummy-token", "verySecret").Return(nil, assert.AnError).Once()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.ResetPassword(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		authUC.AssertExpectations(t)
		logger.AssertExpectations(t)
	})
}

// TestGetUserProfile tests the GetUserProfile handler for retrieving the current user's profile, covering success, missing user in context, and use case errors.
func TestGetUserProfile(t *testing.T) {
	h, logger, authUC := newTestHandler(t)

	t.Run("Successful get user profile", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/user-profile", nil)
		rr := httptest.NewRecorder()
		u := models.User{
			ID:       uuid.New(),
			Name:     "<NAME>",
			Email:    "user@gmail.com",
			Password: "<PASSWORD>",
			Role:     "admin",
		}
		ctx := context.WithValue(req.Context(), UserContextKey, &u)
		req = req.WithContext(ctx)
		authUC.On("GetUserDetails", u.ID).Return(&u, nil).Once()
		h.GetUserProfile(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		authUC.AssertExpectations(t)
	})

	t.Run("No user in context", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/user-profile", nil)
		rr := httptest.NewRecorder()
		logger.On("Error", mock.Anything).Once()
		h.GetUserProfile(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("authUC.GetUserDetails error", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/user-profile", nil)
		rr := httptest.NewRecorder()
		u := models.User{
			ID:       uuid.New(),
			Name:     "<NAME>",
			Email:    "user@gmail.com",
			Password: "<PASSWORD>",
			Role:     "admin",
		}
		ctx := context.WithValue(req.Context(), UserContextKey, &u)
		req = req.WithContext(ctx)
		authUC.On("GetUserDetails", u.ID).Return(nil, assert.AnError).Once()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.GetUserProfile(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		authUC.AssertExpectations(t)
		logger.AssertExpectations(t)
	})
}

// TestUpdatePassword tests the UpdatePassword handler for changing a user's password, covering success, missing user, multipart parsing errors, validation errors, and use case errors.
func TestUpdatePassword(t *testing.T) {
	h, logger, authUC := newTestHandler(t)

	t.Run("Successful update password", func(t *testing.T) {
		formData := url.Values{}
		formData.Add("oldPassword", "verySecretOld")
		formData.Add("password", "verySecret")
		requestBody, ct, _ := utils.CreateMultipartForm(formData)
		req, err := http.NewRequest(http.MethodPost, "/update-password", requestBody)
		require.NoError(t, err)
		req.Header.Set("Content-Type", ct)
		rr := httptest.NewRecorder()
		_ = req.ParseForm()
		passwords := models.Passwords{
			OldPassword: "verySecretOld",
			Password:    "verySecret",
		}
		user := models.User{ID: uuid.New()}
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)
		authUC.On("UpdatePassword", user.ID, passwords).Return(&models.UserResponse{}, nil).Once()
		h.UpdatePassword(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		authUC.AssertExpectations(t)
	})

	t.Run("Missing user in context", func(t *testing.T) {
		formData := url.Values{}
		formData.Add("oldPassword", "verySecretOld")
		formData.Add("password", "verySecret")
		requestBody, ct, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/update-password", requestBody)
		req.Header.Set("Content-Type", ct)
		rr := httptest.NewRecorder()
		logger.On("Error", mock.Anything).Once()
		h.UpdatePassword(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("ParseMultipartForm error", func(t *testing.T) {
		formData := url.Values{}
		formData.Add("oldPassword", "verySecretOld")
		formData.Add("password", "verySecret")
		requestBody := bytes.NewBufferString("")
		req, _ := http.NewRequest(http.MethodPost, "/update-password", requestBody)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		rr := httptest.NewRecorder()
		user := models.User{ID: uuid.New()}
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.UpdatePassword(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("Validation error - missing password", func(t *testing.T) {
		formData := url.Values{}
		formData.Add("oldPassword", "verySecretOld")
		requestBody, ct, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/update-password", requestBody)
		req.Header.Set("Content-Type", ct)
		rr := httptest.NewRecorder()
		user := models.User{ID: uuid.New()}
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.UpdatePassword(rr, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("Validation error - missing oldPassword", func(t *testing.T) {
		formData := url.Values{}
		formData.Add("password", "verySecret")
		requestBody, ct, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/update-password", requestBody)
		req.Header.Set("Content-Type", ct)
		rr := httptest.NewRecorder()
		user := models.User{ID: uuid.New()}
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.UpdatePassword(rr, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("authUC.UpdatePassword error", func(t *testing.T) {
		formData := url.Values{}
		formData.Add("oldPassword", "verySecretOld")
		formData.Add("password", "verySecret")
		requestBody, ct, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/update-password", requestBody)
		req.Header.Set("Content-Type", ct)
		rr := httptest.NewRecorder()
		user := models.User{ID: uuid.New()}
		ctx := context.WithValue(req.Context(), UserContextKey, &user)
		req = req.WithContext(ctx)
		passwords := models.Passwords{
			OldPassword: "verySecretOld",
			Password:    "verySecret",
		}
		authUC.On("UpdatePassword", user.ID, passwords).Return(nil, assert.AnError).Once()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()
		h.UpdatePassword(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		authUC.AssertExpectations(t)
		logger.AssertExpectations(t)
	})
}

// TestUpdateProfile tests the UpdateProfile handler for updating a user's profile, covering success, missing user, multipart parsing errors, validation errors, and use case errors.
func TestUpdateProfile(t *testing.T) {
	h, logger, authUC := newTestHandler(t)

	t.Run("Successful update profile", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("name", "John Doe")
		formData.Set("email", "john.doe@example.com")
		formData.Set("avatar", "newAvatar.jpg")
		body, contentType, err := utils.CreateMultipartForm(formData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/update-profile", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", contentType)
		rr := httptest.NewRecorder()

		u := models.User{
			Name:     "John Doe",
			Email:    "john.doe@example.com",
		}
		ctx := context.WithValue(req.Context(), UserContextKey, &u)
		req = req.WithContext(ctx)

		authUC.On("UpdateProfile", u, "newAvatar.jpg").Return(nil).Once()
		h.UpdateProfile(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		authUC.AssertExpectations(t)
	})

	t.Run("Missing user in context", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("name", "John Doe")
		formData.Set("email", "john.doe@example.com")
		formData.Set("avatar", "newAvatar.jpg")
		body, contentType, err := utils.CreateMultipartForm(formData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "/update-profile", body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", contentType)
		rr := httptest.NewRecorder()

		logger.On("Error", mock.Anything).Once()

		h.UpdateProfile(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("ParseMultipartForm error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/update-profile", nil)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")

		u := models.User{
			Name:     "John Doe",
			Email:    "john.doe@example.com",
		}
		ctx := context.WithValue(req.Context(), UserContextKey, &u)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		logger.On("Errorf", mock.Anything, mock.Anything).Once()

		h.UpdateProfile(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("Validation error - missing name", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("email", "john.doe@example.com")
		formData.Set("avatar", "newAvatar.jpg")

		body, contentType, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/update-profile", body)

		req.Header.Set("Content-Type", contentType)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")

		u := models.User{
			Name:     "John Doe",
			Email:    "john.doe@example.com",
		}
		ctx := context.WithValue(req.Context(), UserContextKey, &u)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()

		logger.On("Errorf", mock.Anything, mock.Anything).Once()

		h.UpdateProfile(rr, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("Validation error - invalid email", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("name", "John Doe")
		formData.Set("email", "invalid-email")
		formData.Set("avatar", "newAvatar.jpg")

		body, contentType, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/update-profile", body)

		req.Header.Set("Content-Type", contentType)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")

		u := models.User{
			Name:     "John Doe",
			Email:    "invalid-email",
		}
		ctx := context.WithValue(req.Context(), UserContextKey, &u)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()

		logger.On("Errorf", mock.Anything, mock.Anything).Once()

		h.UpdateProfile(rr, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)
		logger.AssertExpectations(t)
	})

	t.Run("authUC.UpdateProfile error", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("name", "John Doe")
		formData.Set("email", "john.doe@example.com")
		formData.Set("avatar", "newAvatar.jpg")

		body, contentType, _ := utils.CreateMultipartForm(formData)
		req, _ := http.NewRequest(http.MethodPost, "/update-profile", body)

		req.Header.Set("Content-Type", contentType)
		rCtx := chi.NewRouteContext()
		rCtx.URLParams.Add("token", "dummy-token")

		u := models.User{
			Name:     "John Doe",
			Email:    "john.doe@example.com",
		}
		ctx := context.WithValue(req.Context(), UserContextKey, &u)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()

		authUC.On("UpdateProfile", mock.Anything, mock.Anything).Return(assert.AnError).Once()
		logger.On("Errorf", mock.Anything, mock.Anything).Once()

		h.UpdateProfile(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		authUC.AssertExpectations(t)
		logger.AssertExpectations(t)
	})
}
