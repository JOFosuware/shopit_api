package auth

import (
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

type Repo interface {
	// InsertUser inserts a user into the user table
	InsertUser(user models.User) (*models.User, error)

	// InsertAvatar insert avatar resource locator
	InsertAvatar(avatar *models.Avatar) (models.Avatar, error)

	// InsertToken inserts a token into the tokens table
	InsertToken(t *models.Token, userID uuid.UUID) error

	// FetchTokenById fetches a token by id
	FetchTokenById(id uuid.UUID) (*models.Token, error)

	// FetchAvatarById fetches avatar data using user id from the avatar table
	FetchAvatarById(userId uuid.UUID) (models.Avatar, error)

	// DeleteUsers deletes the users from the database
	DeleteUsers() error

	// DeleteAvatar deletes the avatar from the database
	DeleteAvatar() error

	// FetchUserByEmail fetches a user by email from the database
	FetchUserByEmail(email string) (*models.User, error)

	// FetchUserByToken fetches a user by token
	FetchUserByToken(token string) (*models.User, error)

	// UpdateUser updates the users table with new changes
	UpdateUser(user models.User) error

	// FetchUserById returns a user by id and error if any error occurs
	FetchUserById(id uuid.UUID) (*models.User, error)

	// DeleteAvatarById deletes an avatar by id
	DeleteAvatarById(id string) error

	// FetchAllUsers returns all users and error if any error occurs
	FetchAllUsers() ([]*models.User, error)

	// DeleteUserById deletes a user by id and error if any error occurs
	DeleteUserById(id uuid.UUID) error

	// DeleteTokenById deletes a token by user id and error if any error occurs
	DeleteTokenById(userId uuid.UUID) error
}
