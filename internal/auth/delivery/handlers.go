// Package delivery provides HTTP handlers for authentication-related endpoints.
// These handlers implement user registration, login, password management, profile management,
// and admin user management for the authentication service.
package delivery

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/auth"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/pkg/logger"
	"github.com/jofosuware/go/shopit/pkg/utils"
	"github.com/jofosuware/go/shopit/pkg/validator"
)

const UserContextKey = utils.UserContextKey

// AuthHandlers provides HTTP handler methods for authentication endpoints.
// It depends on a logger and an AuthenticateUC usecase interface for business logic.
type AuthHandlers struct {
	logger logger.Logger // Logger for error/info logging
	authUC auth.AuthenticateUC // Usecase interface for authentication business logic
}

// NewAuthHandlers constructs a new AuthHandlers instance.
//
// Parameters:
//   - logger: Logger for logging errors and info
//   - authUC: AuthenticateUC usecase for authentication business logic
//
// Returns:
//   - *AuthHandlers: a new AuthHandlers instance
func NewAuthHandlers(
	logger logger.Logger,
	authUC auth.AuthenticateUC,
) *AuthHandlers {
	return &AuthHandlers{
		logger: logger,
		authUC: authUC,
	}
}

// Register handles user registration requests.
// Endpoint: POST /api/v1/auth/register
//
// Expects multipart form data with fields: name, email, password, avatar.
// Validates input, calls the usecase, and returns a JSON response.
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(100000)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("parsing multipart form error: %v", err)
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
	v.Check(len(password) > 7, "password", "password must be at least 8 characters")
	v.Check(avatar != "", "avatar", "user avatar must be provided")

	if !v.Valid() {
		utils.FailedValidation(w, r, v.Errors)
		h.logger.Errorf("Failed validation: %v", v.Errors)
		return
	}

	u := models.User{
		Name:     name,
		Email:    email,
		Password: password,
		Role:     "user",
	}

	res, err := h.authUC.Register(u, avatar)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("error registering user"))
		h.logger.Errorf("Error registering user: %v", err)
		return
	}

	if err := utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("writing json error: %v", err)
		return
	}

}

// Login handles user login requests.
// Endpoint: POST /api/v1/auth/login
//
// Expects JSON body with fields: email, password.
// Validates input, calls the usecase, and returns a JSON response.
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var u *models.User

	// Parse json into user struct
	err := utils.ReadJSON(w, r, &u)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("reading json error: %v", err)
		return
	}

	// validate data
	v := validator.New()
	v.Check(u.Email != "", "email", "user email must be provided")
	v.Check(len(u.Password) > 7, "password", "password must be at least 8 characters")

	if !v.Valid() {
		utils.FailedValidation(w, r, v.Errors)
		h.logger.Errorf("Failed validation: %v", v.Errors)
		return
	}

	res, err := h.authUC.Login(u.Email, u.Password)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("error logging in user, invalid user or user does not exists"))
		h.logger.Errorf("Error logging in user: %v", err)
		return
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}

}

// SendPasswordResetEmail handles requests to send a password reset email.
// Endpoint: POST /api/v1/auth/password/forgot
//
// Expects multipart form data with field: email.
// Validates input, calls the usecase, and returns a JSON response.
func (h *AuthHandlers) SendPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10000)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("parsing form error: %v", err)
		return
	}

	email := r.Form.Get("email")

	// validate data
	v := validator.New()
	v.Check(email != "", "email", "user email must be provided")

	if !v.Valid() {
		utils.FailedValidation(w, r, v.Errors)
		h.logger.Errorf("Failed validation: %v", v.Errors)
		return
	}

	res, err := h.authUC.SendPasswordResetEmail(email, r)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("Error sending password reset email: %v", err)
		return
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}

}

// ResetPassword handles password reset requests.
// Endpoint: POST /api/v1/auth/password/reset/:token
//
// Expects multipart form data with fields: password, confirmPassword.
// Validates input, checks password match, calls the usecase, and returns a JSON response.
func (h *AuthHandlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	t := chi.URLParam(r, "token")

	err := r.ParseMultipartForm(10000)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("parsing form error: %v", err)
		return
	}

	password := r.Form.Get("password")
	confirm := r.Form.Get("confirmPassword")

	// validate data
	v := validator.New()
	v.Check(password != "", "password", "password must be provided")
	v.Check(confirm != "", "confirmPassword", "confirm password must be provided")

	if !v.Valid() {
		utils.FailedValidation(w, r, v.Errors)
		h.logger.Errorf("Failed validation: %v", v.Errors)
		return
	}

	if password != confirm {
		_ = utils.BadRequest(w, r, errors.New("passwors mismatch"))
		h.logger.Info("Passwords mismatch")
		return
	}

	res, err := h.authUC.ResetPassword(t, password)
	if err != nil {
		_ = utils.BadRequest(w, r, errors.New("password reset unsuccessful, try again later"))
		h.logger.Errorf("Error resetting password: %v", err)
		return
	}

	if err := utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("writing json error: %v", err)
		return
	}
}

// GetUserProfile returns the currently logged-in user's profile.
// Endpoint: GET /api/v1/auth/me
//
// Requires user context to be set (e.g., by authentication middleware).
func (h *AuthHandlers) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		_ = utils.BadRequest(w, r, errors.New(""))
		h.logger.Error("unable to retrieve user from session")
		return
	}

	// Fetch user details
	user, err := h.authUC.GetUserDetails(user.ID)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error getting user details: %v", err)
		return
	}

	jr := models.UserResponse{
		Success: true,
		User:    *user,
	}

	if err := utils.WriteJSON(w, http.StatusOK, jr); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// UpdatePassword updates the password for the currently logged-in user.
// Endpoint: POST /api/v1/auth/password/update
//
// Expects multipart form data with fields: oldPassword, password.
// Validates input, calls the usecase, and returns a JSON response.
func (h *AuthHandlers) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		_ = utils.BadRequest(w, r, errors.New(""))
		h.logger.Error("unable to retrieve user from session")
		return
	}

	err := r.ParseMultipartForm(10000)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("parsing form error: %v", err)
		return
	}

	password := r.Form.Get("password")
	oldPassword := r.Form.Get("oldPassword")

	// validate data
	v := validator.New()
	v.Check(password != "", "password", "password must be provided")
	v.Check(oldPassword != "", "oldPassword", "old password must be provided")

	if !v.Valid() {
		utils.FailedValidation(w, r, v.Errors)
		h.logger.Errorf("Failed validation: %v", v.Errors)
		return
	}

	passwords := models.Passwords{
		Password:    password,
		OldPassword: oldPassword,
	}

	res, err := h.authUC.UpdatePassword(user.ID, passwords)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("Error updating password: %v", err)
		return
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// UpdateProfile updates the profile of the currently logged-in user.
// Endpoint: POST /api/v1/auth/me/update
//
// Expects multipart form data with fields: name, email, avatar.
// Validates input, calls the usecase, and returns a JSON response.
func (h *AuthHandlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(UserContextKey).(*models.User)
	if !ok {
		_ = utils.BadRequest(w, r, errors.New(""))
		h.logger.Error("unable to retrieve user from session")
		return
	}

	err := r.ParseMultipartForm(10000)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("parsing form error: %v", err)
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
		utils.FailedValidation(w, r, v.Errors)
		h.logger.Errorf("Failed validation: %v", v.Errors)
		return
	}

	user.Name = name
	user.Email = email

	err = h.authUC.UpdateProfile(*user, avatar)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("Error updating profile: %v", err)
		return
	}

	res := struct {
		Success bool `json:"success"`
	}{
		Success: true,
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// Logout logs out the user by deleting their token.
// Endpoint: POST /api/v1/auth/logout
//
// Expects URL param: token.
// Calls the usecase to delete the token and returns a JSON response.
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	t := chi.URLParam(r, "token")

	if t == "" {
		_ = utils.BadRequest(w, r, errors.New("token must be provided"))
		h.logger.Error("token must be provided")
		return
	}

	err := h.authUC.DeleteUserToken(t)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error deleting user token: %v", err)
		return
	}

	res := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "logout successful",
	}

	if err := utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// GetAllUsers returns all users (admin only).
// Endpoint: GET /api/v1/auth/admin/users
//
// Calls the usecase to fetch all users and returns a JSON response.
func (h *AuthHandlers) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.authUC.GetAllUsers()
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error getting all users: %v", err)
		return
	}

	res := struct {
		Success bool           `json:"success"`
		Users   []*models.User `json:"users"`
	}{
		Success: true,
		Users:   users,
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// GetUserDetails returns details for a specific user (admin only).
// Endpoint: GET /api/v1/auth/admin/user/:id
//
// Expects URL param: id (UUID).
// Calls the usecase to fetch user details and returns a JSON response.
func (h *AuthHandlers) GetUserDetails(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		_ = utils.BadRequest(w, r, errors.New("id must be provided"))
		h.logger.Error("id must be provided")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error parsing id: %v", err)
		return
	}

	user, err := h.authUC.GetUserDetails(userID)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error getting user details: %v", err)
		return
	}

	res := models.UserResponse{
		Success: true,
		User:    *user,
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// UpdateUser updates a user's profile (admin only).
// Endpoint: PUT /api/v1/auth/admin/user/:id
//
// Expects URL param: id (UUID) and multipart form data with fields: name, email, role.
// Validates input, calls the usecase, and returns a JSON response.
func (h *AuthHandlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		_ = utils.BadRequest(w, r, errors.New("id must be provided"))
		h.logger.Error("id must be provided")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error parsing id: %v", err)
		return
	}

	err = r.ParseMultipartForm(100000)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error parsing multipart form: %v", err)
		return
	}

	name := r.Form.Get("name")
	email := r.Form.Get("email")
	role := r.Form.Get("role")

	fmt.Printf("Name: %s and email: %s", name, email)

	v := validator.New()

	v.Check(name != "", "name", "user name must be provided")
	v.Check(email != "", "email", "user email must be provided")

	if !v.Valid() {
		_ = utils.BadRequest(w, r, errors.New("invalid input"))
		h.logger.Error("invalid input")
		return
	}

	user := models.User{
		Name:  name,
		Email: email,
		Role:  role,
	}

	res, err := h.authUC.UpdateUser(userID, user)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error updating user: %v", err)
		return
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}

// DeleteUser deletes a user (admin only).
// Endpoint: DELETE /api/v1/auth/admin/user/:id
//
// Expects URL param: id (UUID).
// Calls the usecase to delete the user and returns a JSON response.
func (h *AuthHandlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		_ = utils.BadRequest(w, r, errors.New("id must be provided"))
		h.logger.Error("id must be provided")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error parsing id: %v", err)
		return
	}

	err = h.authUC.DeleteUser(userID)
	if err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error deleting user: %v", err)
		return
	}

	res := models.UserResponse{
		Success: true,
	}

	if err = utils.WriteJSON(w, http.StatusOK, res); err != nil {
		_ = utils.BadRequest(w, r, err)
		h.logger.Errorf("error writing json: %v", err)
		return
	}
}
