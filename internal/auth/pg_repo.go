package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type Repo interface {
	// CreateUserAtomically inserts User, Avatar, and Token within a single database transaction.
	// This ensures that the database state is consistent across all necessary write operations.
	CreateUserAtomically(ctx context.Context, user models.User, avatar models.Avatar, t *models.Token) (*models.User, error)

	// InsertToken inserts a token into the tokens table.
	InsertToken(ctx context.Context, t *models.Token, userID uuid.UUID) error

	// FetchAvatarById fetches avatar data using user id from the avatar table
	FetchAvatarById(ctx context.Context, userId uuid.UUID) (models.Avatar, error)

	// FetchUserByEmail fetches a user by email from the database
	FetchUserByEmail(ctx context.Context, email string) (*models.User, error)

	// FetchUserByTokenWithScope fetches a user by token and required scope
	FetchUserByTokenWithScope(ctx context.Context, token string, scope string) (*models.User, error)

	// UpdateUser updates the users table with new changes
	UpdateUser(ctx context.Context, user models.User) error

	// UpdatePasswordAndReplaceToken atomically changes a password, replaces the
	// authentication token, and optionally invalidates password-reset tokens.
	UpdatePasswordAndReplaceToken(ctx context.Context, user models.User, token *models.Token, revokeResetTokens bool) error

	// FetchUserById returns a user by id and error if any error occurs
	FetchUserById(ctx context.Context, id uuid.UUID) (*models.User, error)

	// FetchAllUsers returns public user data.
	FetchAllUsers(ctx context.Context) ([]*models.UserPublic, error)

	// DeleteUserById deletes a user by id and error if any error occurs
	DeleteUserById(ctx context.Context, id uuid.UUID) error

	// DeleteAuthenticationToken deletes the presented authentication token.
	DeleteAuthenticationToken(ctx context.Context, plainTextToken string) error

	// UpdateProfile persists profile fields and an optional avatar replacement atomically.
	UpdateProfile(ctx context.Context, userID uuid.UUID, name, email string, avatar *models.Avatar) error
}
