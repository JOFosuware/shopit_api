// Package delivery provides HTTP routes for authentication endpoints.
//
// It wires handler methods for registration, login, password management,
// profile management, and admin user management, applying authentication
// middleware where appropriate.
package delivery

import (
	"net/http"

	"github.com/jofosuware/go/shopit/pkg/utils"

	"github.com/go-chi/chi/v5"
)

// AuthRouter returns a chi.Router configured with authentication and
// user-management routes.
//
// Public routes:
//   - POST   /register                → Register a new user
//   - POST   /login                   → Login a user
//   - POST   /password/forgot         → Send password reset email
//   - PUT    /password/reset/{token}  → Reset password with token
//   - GET    /logout/{token}          → Logout user (delete token)
//
// Authenticated routes (require IsAuthenticated middleware):
//   - GET    /me                      → Get current user profile
//   - PUT    /password/update         → Update current user password
//   - PUT    /me/update               → Update current user profile
//   - GET    /admin/users             → Get all users (admin)
//   - GET    /admin/user/{id}         → Get user details by ID (admin)
//   - PUT    /admin/user/{id}         → Update user by ID (admin)
//   - DELETE /admin/user/{id}         → Delete user by ID (admin)
func (h *AuthHandlers) AuthRouter() http.Handler {
	mux := chi.NewRouter()

	mux.Post("/register", h.Register)
	mux.Post("/login", h.Login)
	mux.Post("/password/forgot", h.SendPasswordResetEmail)
	mux.Put("/password/reset/{token}", h.ResetPassword)

	mux.Get("/logout/{token}", h.Logout)

	mux.Group(func(r chi.Router) {
		r.Use(utils.IsAuthenticated)

		r.Get("/me", h.GetUserProfile)
		r.Put("/password/update", h.UpdatePassword)
		r.Put("/me/update", h.UpdateProfile)
		r.Get("/admin/users", h.GetAllUsers)
		r.Get("/admin/user/{id}", h.GetUserDetails)
		r.Put("/admin/user/{id}", h.UpdateUser)
		r.Delete("/admin/user/{id}", h.DeleteUser)
	})

	return mux
}
