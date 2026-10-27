// Package delivery provides HTTP handlers for authentication-related endpoints.
// These handlers implement user registration, login, password management, profile management,
// and admin user management for the authentication service.
package delivery

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/auth"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/pkg/logger"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"github.com/jofosuware/go/shopit/pkg/validator"
)

// UserContextKey is the request context key used to store the authenticated user.
const UserContextKey = utils.UserContextKey

// AuthHandlers provides HTTP handler methods for authentication endpoints.
// It depends on a logger and an AuthenticateUC usecase interface for business logic.
type AuthHandlers struct {
	logger logger.Logger
	authUC auth.AuthenticateUC
}

// NewAuthHandlers returns a new AuthHandlers with the provided logger and usecase.
func NewAuthHandlers(
	logger logger.Logger,
	authUC auth.AuthenticateUC,
) *AuthHandlers {
	return &AuthHandlers{
		logger: logger,
		authUC: authUC,
	}
}

// Register registers a new user.
// Endpoint: POST /api/v1/auth/register
// Expects multipart form data: name, email, password, avatar.
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(100000)
	if err != nil {
		h.writeBadRequest(w, "parsing registration form", err)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	avatar := r.FormValue("avatar")

	// validate data
	v := validator.New()
	v.Check(name != "", "name", "user name must be provided")
	v.Check(email != "", "email", "user email must be provided")
	v.IsEmailValid(email, "email", "user email must be valid")
	v.Check(len(password) > 7, "password", "password must be at least 8 characters")
	v.Check(avatar != "", "avatar", "user avatar must be provided")

	if !v.Valid() {
		h.writeValidationError(w, r, "validating registration", v.Errors)
		return
	}

	u := models.User{
		Name:     name,
		Email:    email,
		Password: password,
		Role:     "user",
	}

	res, err := h.authUC.Register(r.Context(), u, avatar)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserAlreadyExists):
			h.writeError(w, http.StatusConflict, "user already exists")
			h.logger.Errorf("registering user: %v", err)
		case errors.Is(err, auth.ErrAvatarUploadFailed):
			h.logger.Errorf("uploading registration avatar: %v", err)
			h.writeError(w, http.StatusBadGateway, "failed to upload avatar")
		default:
			h.writeInternalError(w, "registering user", err)
		}
		return
	}

	if err := h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}

}

// Login authenticates a user and returns a token.
// Endpoint: POST /api/v1/auth/login
// Expects JSON body: email, password.
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// Parse JSON into a dedicated login request payload rather than a full user model.
	err := utils.ReadJSON(w, r, &req)
	if err != nil {
		h.writeBadRequest(w, "reading login request", err)
		return
	}

	// validate data
	v := validator.New()
	v.Check(req.Email != "", "email", "user email must be provided")
	v.Check(len(req.Password) > 7, "password", "password must be at least 8 characters")

	if !v.Valid() {
		h.writeValidationError(w, r, "validating login", v.Errors)
		return
	}

	res, err := h.authUC.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) || errors.Is(err, auth.ErrInvalidPassword) {
			h.writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		h.writeInternalError(w, "logging in user", err)
		return
	}

	if err = h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}

}

// SendPasswordResetEmail sends a password reset email.
// Endpoint: POST /api/v1/auth/password/forgot
// Expects form data: email.
func (h *AuthHandlers) SendPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10000)
	if err != nil {
		h.writeBadRequest(w, "parsing password-reset request", err)
		return
	}

	email := r.Form.Get("email")

	// validate data
	v := validator.New()
	v.Check(email != "", "email", "user email must be provided")
	v.IsEmailValid(email, "email", "user email must be valid")

	if !v.Valid() {
		h.writeValidationError(w, r, "validating password-reset request", v.Errors)
		return
	}

	_, err = h.authUC.SendPasswordResetEmail(r.Context(), email)
	if err != nil {
		h.logger.Errorf("processing password reset request: %v", err)
	}

	h.writePasswordResetAccepted(w)

}

func (h *AuthHandlers) writePasswordResetAccepted(w http.ResponseWriter) {
	h.writeJSON(w, http.StatusAccepted, models.Response{
		Success: true,
		Message: "If an account exists for that email, password reset instructions will be sent.",
	})
}

// ResetPassword resets a user's password using a reset token.
// Endpoint: PUT /api/v1/auth/password/reset/{token}
// Expects form data: password, confirmPassword.
func (h *AuthHandlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	t := chi.URLParam(r, "token")
	if t == "" {
		h.writeError(w, http.StatusBadRequest, "password reset token must be provided")
		return
	}

	err := r.ParseMultipartForm(10000)
	if err != nil {
		h.writeBadRequest(w, "parsing password-reset form", err)
		return
	}

	password := r.Form.Get("password")
	confirm := r.Form.Get("confirmPassword")

	// validate data
	v := validator.New()
	v.Check(password != "", "password", "password must be provided")
	v.Check(len(password) > 7, "password", "password must be at least 8 characters")
	v.Check(confirm != "", "confirmPassword", "confirm password must be provided")

	if !v.Valid() {
		h.writeValidationError(w, r, "validating password reset", v.Errors)
		return
	}

	if password != confirm {
		h.writeError(w, http.StatusUnprocessableEntity, "passwords do not match")
		h.logger.Info("Passwords mismatch")
		return
	}

	res, err := h.authUC.ResetPassword(r.Context(), t, password)
	if err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			h.writeError(w, http.StatusBadRequest, "password reset link is invalid or expired")
			return
		}
		h.writeInternalError(w, "resetting password", err)
		return
	}

	if err := h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}
}

// GetUserProfile returns the profile of the authenticated user.
// Endpoint: GET /api/v1/auth/me
func (h *AuthHandlers) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	actor, ok := authenticatedUser(r.Context())
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		h.logger.Error("unable to retrieve user from session")
		return
	}

	// Fetch user details
	user, err := h.authUC.GetUserDetails(r.Context(), actor, actor.ID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			h.writeError(w, http.StatusNotFound, "user not found")
		} else if errors.Is(err, auth.ErrUserAccessDenied) || errors.Is(err, auth.ErrForbidden) {
			h.writeError(w, http.StatusForbidden, "access denied")
		} else {
			h.writeInternalError(w, "getting user details", err)
		}
		return
	}

	jr := models.UserResponse{
		Success: true,
		User:    *user,
	}

	if err := h.writeJSON(w, http.StatusOK, jr); err != nil {
		return
	}
}

// UpdatePassword updates the authenticated user's password.
// Endpoint: PUT /api/v1/auth/password/update
// Expects form data: oldPassword, password.
func (h *AuthHandlers) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r.Context())
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		h.logger.Error("unable to retrieve user from session")
		return
	}

	err := r.ParseMultipartForm(10000)
	if err != nil {
		h.writeBadRequest(w, "parsing password-update form", err)
		return
	}

	password := r.Form.Get("password")
	oldPassword := r.Form.Get("oldPassword")

	// validate data
	v := validator.New()
	v.Check(password != "", "password", "password must be provided")
	v.Check(len(password) > 7, "password", "password must be at least 8 characters")
	v.Check(oldPassword != "", "oldPassword", "old password must be provided")

	if !v.Valid() {
		h.writeValidationError(w, r, "validating password update", v.Errors)
		return
	}

	passwords := models.Passwords{
		Password:    password,
		OldPassword: oldPassword,
	}

	res, err := h.authUC.UpdatePassword(r.Context(), user.ID, passwords)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidOldPassword) {
			h.writeError(w, http.StatusUnauthorized, "current password is incorrect")
		} else if errors.Is(err, auth.ErrUserNotFound) {
			h.writeError(w, http.StatusNotFound, "user not found")
		} else {
			h.writeInternalError(w, "updating password", err)
		}
		return
	}

	if err = h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}
}

// UpdateProfile updates the authenticated user's profile and avatar.
// Endpoint: PUT /api/v1/auth/me/update
// Expects form data: name, email, avatar.
func (h *AuthHandlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r.Context())
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		h.logger.Error("unable to retrieve user from session")
		return
	}

	err := r.ParseMultipartForm(10000)
	if err != nil {
		h.writeBadRequest(w, "parsing profile-update form", err)
		return
	}

	name := r.Form.Get("name")
	email := r.Form.Get("email")
	avatar := r.FormValue("avatar")

	// validate data
	v := validator.New()

	v.Check(name != "", "name", "name must be provided")
	v.Check(email != "", "email", "email must be provided")
	v.IsEmailValid(email, "email", "email must be valid")

	if !v.Valid() {
		h.writeValidationError(w, r, "validating profile update", v.Errors)
		return
	}

	user.Name = name
	user.Email = email

	err = h.authUC.UpdateProfile(r.Context(), *user, avatar)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			h.writeError(w, http.StatusNotFound, "user not found")
		} else if errors.Is(err, auth.ErrUserAlreadyExists) {
			h.writeError(w, http.StatusConflict, "email address is already in use")
		} else if errors.Is(err, auth.ErrAvatarUploadFailed) {
			h.logger.Errorf("uploading profile avatar: %v", err)
			h.writeError(w, http.StatusBadGateway, "failed to upload avatar")
		} else if errors.Is(err, auth.ErrAvatarDeletionFailed) {
			h.logger.Errorf("cleaning up old profile avatar: %v", err)
			h.writeError(w, http.StatusBadGateway, "profile updated, but avatar cleanup failed")
		} else {
			h.writeInternalError(w, "updating profile", err)
		}
		return
	}

	res := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	if err = h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}
}

// Logout deletes the provided token and logs out the user.
// Endpoint: POST /api/v1/auth/logout
// Expects an Authorization bearer token.
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	t := bearerToken(r)

	if t == "" {
		h.writeError(w, http.StatusBadRequest, "authorization bearer token must be provided")
		return
	}

	err := h.authUC.DeleteUserToken(r.Context(), t)
	if err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			h.writeError(w, http.StatusUnauthorized, "invalid authentication credentials")
		} else {
			h.writeInternalError(w, "deleting user token", err)
		}
		return
	}

	res := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "logout successful",
	}

	if err := h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}
}

func bearerToken(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// GetAllUsers returns all users (admin).
// Endpoint: GET /api/v1/auth/admin/users
func (h *AuthHandlers) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	actor, ok := authenticatedUser(r.Context())
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		h.logger.Error("unable to retrieve user from session")
		return
	}

	users, err := h.authUC.GetAllUsers(r.Context(), actor)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			h.writeError(w, http.StatusForbidden, "forbidden: admin only")
		} else {
			h.writeInternalError(w, "getting all users", err)
		}
		return
	}

	res := struct {
		Success bool                 `json:"success"`
		Users   []*models.UserPublic `json:"users"`
	}{
		Success: true,
		Users:   users,
	}

	if err = h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}
}

// GetUserDetails returns details for a specific user (admin).
// Endpoint: GET /api/v1/auth/admin/user/{id}
// Expects URL param: id (UUID).
func (h *AuthHandlers) GetUserDetails(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id must be provided")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing user ID", err)
		return
	}

	actor, ok := authenticatedUser(r.Context())
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		h.logger.Error("unable to retrieve user from session")
		return
	}

	user, err := h.authUC.GetUserDetails(r.Context(), actor, userID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			h.writeError(w, http.StatusNotFound, "user not found")
		case errors.Is(err, auth.ErrUserAccessDenied), errors.Is(err, auth.ErrForbidden):
			h.writeError(w, http.StatusForbidden, "access denied")
		default:
			h.writeInternalError(w, "getting user details", err)
		}
		return
	}

	res := models.UserResponse{
		Success: true,
		User:    *user,
	}

	if err = h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}
}

// UpdateUser updates a user's profile (admin).
// Endpoint: PUT /api/v1/auth/admin/user/{id}
// Expects URL param: id (UUID) and form data: name, email, role.
func (h *AuthHandlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id must be provided")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing user ID", err)
		return
	}

	err = r.ParseMultipartForm(100000)
	if err != nil {
		h.writeBadRequest(w, "parsing user-update form", err)
		return
	}

	name := r.Form.Get("name")
	email := r.Form.Get("email")
	role := r.Form.Get("role")

	v := validator.New()

	v.Check(name != "", "name", "user name must be provided")
	v.Check(email != "", "email", "user email must be provided")
	v.IsEmailValid(email, "email", "user email must be valid")
	v.Check(role == "admin" || role == "user", "role", "role must be either admin or user")

	if !v.Valid() {
		h.writeValidationError(w, r, "validating user update", v.Errors)
		return
	}

	user := models.User{
		Name:  name,
		Email: email,
		Role:  role,
	}

	actor, ok := authenticatedUser(r.Context())
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		h.logger.Error("unable to retrieve user from session")
		return
	}

	res, err := h.authUC.UpdateUser(r.Context(), actor, userID, user)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			h.writeError(w, http.StatusNotFound, "user not found")
		case errors.Is(err, auth.ErrForbidden):
			h.writeError(w, http.StatusForbidden, "forbidden: admin only")
		case errors.Is(err, auth.ErrUserAlreadyExists):
			h.writeError(w, http.StatusConflict, "email address is already in use")
		default:
			h.writeInternalError(w, "updating user", err)
		}
		return
	}

	if err = h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}
}

// DeleteUser deletes a user (admin).
// Endpoint: DELETE /api/v1/auth/admin/user/{id}
// Expects URL param: id (UUID).
func (h *AuthHandlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id must be provided")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		h.writeBadRequest(w, "parsing user ID", err)
		return
	}

	actor, ok := authenticatedUser(r.Context())
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		h.logger.Error("unable to retrieve user from session")
		return
	}

	err = h.authUC.DeleteUser(r.Context(), actor, userID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			h.writeError(w, http.StatusNotFound, "user not found")
		case errors.Is(err, auth.ErrForbidden):
			h.writeError(w, http.StatusForbidden, "forbidden: admin only")
		case errors.Is(err, auth.ErrAvatarDeletionFailed):
			h.logger.Errorf("cleaning up deleted user's avatar: %v", err)
			h.writeError(w, http.StatusBadGateway, "user was deleted, but avatar cleanup failed")
		default:
			h.writeInternalError(w, "deleting user", err)
		}
		return
	}

	res := models.UserResponse{
		Success: true,
	}

	if err = h.writeJSON(w, http.StatusOK, res); err != nil {
		return
	}
}
