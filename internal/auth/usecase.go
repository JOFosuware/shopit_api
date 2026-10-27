package auth

import (
	"context"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type AuthenticateUC interface {
	// Authenticate resolves an unexpired authentication token to its user.
	Authenticate(ctx context.Context, token string) (*models.User, error)

	// Register signup a user
	Register(ctx context.Context, user models.User, avatar string) (*models.UserResponse, error)

	// Login login a user
	Login(ctx context.Context, email, password string) (*models.UserResponse, error)

	// SendPasswordResetEmail process password and email reset
	SendPasswordResetEmail(ctx context.Context, email string) (*models.Response, error)

	// ResetPassword reset password
	ResetPassword(ctx context.Context, token, password string) (*models.UserResponse, error)

	// UpdatePassword update password for a user by id
	UpdatePassword(ctx context.Context, userId uuid.UUID, passwords models.Passwords) (*models.UserResponse, error)

	// UpdateProfile update a user profile, returns error on failure
	UpdateProfile(ctx context.Context, user models.User, avatar string) error

	// GetAllUsers fetches public user data for the authenticated caller.
	GetAllUsers(ctx context.Context, actor *models.User) ([]*models.UserPublic, error)

	// GetUserDetails fetches public user data based on the provided userID.
	// The `actor` is the authenticated caller.
	GetUserDetails(ctx context.Context, actor *models.User, userID uuid.UUID) (*models.UserPublic, error)

	// UpdateUser updates the user data in the database based on the provided userID and returns
	// a pointer to the updated UserResponse struct or an error if any occurs during the process.
	// The `actor` is the authenticated caller.
	UpdateUser(ctx context.Context, actor *models.User, userID uuid.UUID, user models.User) (*models.UserResponse, error)

	// DeleteUser deletes the user data from the database based on the provided userID and returns
	// an error if any occurs during the process. The `actor` is the authenticated caller.
	DeleteUser(ctx context.Context, actor *models.User, userID uuid.UUID) error

	// DeleteUserToken deletes the user token from the database based on the provided userID and returns an error
	// if any occurs during the process.
	DeleteUserToken(ctx context.Context, token string) error
}
