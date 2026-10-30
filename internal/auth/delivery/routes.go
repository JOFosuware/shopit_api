// Package delivery provides HTTP routes for authentication endpoints.
//
// It wires handler methods for registration, login, password management,
// profile management, and admin user management, applying authentication
// middleware where appropriate.
package delivery

import (
	"net/http"

	"github.com/jofosuware/go/shopit/pkg/observability"
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
//
// Authenticated routes require a valid authentication-scope bearer token.
//   - GET    /me                      → Get current user profile
//   - POST   /logout                  → Logout user (revoke presented token)
//   - PUT    /password/update         → Update current user password
//   - PUT    /me/update               → Update current user profile
//   - GET    /admin/users             → Get all users (admin)
//   - GET    /admin/user/{id}         → Get user details by ID (admin)
//   - PUT    /admin/user/{id}         → Update user by ID (admin)
//   - DELETE /admin/user/{id}         → Delete user by ID (admin)
func (h *AuthHandlers) AuthRouter() http.Handler {
	mux := chi.NewRouter()

	mux.With(h.registerRateLimiter.Middleware).Post("/register", observability.Handler("auth.handler.register", h.Register))
	mux.With(h.loginRateLimiter.Middleware).Post("/login", observability.Handler("auth.handler.login", h.Login))
	mux.With(h.forgotPasswordRateLimiter.Middleware).Post("/password/forgot", observability.Handler("auth.handler.password_reset_request", h.SendPasswordResetEmail))
	mux.With(h.resetPasswordRateLimiter.Middleware).Put("/password/reset/{token}", observability.Handler("auth.handler.password_reset", h.ResetPassword))

	mux.Group(func(r chi.Router) {
		r.Use(h.authenticate)

		r.Get("/me", observability.Handler("auth.handler.profile", h.GetUserProfile))
		r.Post("/logout", observability.Handler("auth.handler.logout", h.Logout))
		r.Put("/password/update", observability.Handler("auth.handler.update_password", h.UpdatePassword))
		r.Put("/me/update", observability.Handler("auth.handler.update_profile", h.UpdateProfile))

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(utils.AdminOnly)
			r.Get("/admin/users", observability.Handler("auth.handler.list_users", h.GetAllUsers))
			r.Get("/admin/user/{id}", observability.Handler("auth.handler.get_user", h.GetUserDetails))
			r.Put("/admin/user/{id}", observability.Handler("auth.handler.update_user", h.UpdateUser))
			r.Delete("/admin/user/{id}", observability.Handler("auth.handler.delete_user", h.DeleteUser))
		})
	})

	return mux
}
