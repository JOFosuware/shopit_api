// Package repository provides the data access layer for authentication-related operations.
// It implements methods for user, avatar, and token management using a SQL database.
package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

// AuthRepository provides methods for interacting with the authentication-related tables in the database.
// It should be constructed with a *sql.DB connection.
type AuthRepository struct {
	DB *sql.DB // Database connection
}

// NewAuthRepository constructs a new AuthRepository.
//
// Parameters:
//   - db: a pointer to a sql.DB database connection
//
// Returns:
//   - *AuthRepository: a new AuthRepository instance
func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{
		DB: db,
	}
}

// InsertUser inserts a new user into the users table.
// Returns the created user or an error.
func (r *AuthRepository) InsertUser(u models.User) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user models.User

	query := `insert into users (name, email, password, role, created_at) values ($1, $2, $3, $4, $5) returning user_id, name, email, password, role, created_at`

	err := r.DB.QueryRowContext(ctx, query,
		u.Name,
		u.Email,
		u.Password,
		u.Role,
		time.Now(),
	).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.CreatedAt,
	)

	if err != nil {
		return &user, err
	}

	return &user, nil
}

// UpdateUser updates an existing user in the users table.
// Returns an error if the update fails.
func (r *AuthRepository) UpdateUser(u models.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `update users set name = $1, email = $2, password = $3, role = $4 where user_id = $5`

	_, err := r.DB.ExecContext(ctx, query,
		u.Name,
		u.Email,
		u.Password,
		u.Role,
		u.ID,
	)

	if err != nil {
		return err
	}

	return nil
}

// InsertAvatar inserts a new avatar record for a user.
// Returns the created avatar or an error.
func (r *AuthRepository) InsertAvatar(a *models.Avatar) (models.Avatar, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var avatar models.Avatar

	query := `
		insert into avatar 
			(public_id, url, user_id)
		values
			($1, $2, $3)
		returning public_id, url, user_id
	`
	err := r.DB.QueryRowContext(ctx, query,
		&a.PublicId,
		&a.Url,
		&a.UserId,
	).Scan(
		&avatar.PublicId,
		&avatar.Url,
		&avatar.UserId,
	)

	if err != nil {
		return avatar, err
	}

	return avatar, nil
}

// FetchAvatarById fetches an avatar by user ID.
// Returns the avatar or an error.
func (r *AuthRepository) FetchAvatarById(userId uuid.UUID) (models.Avatar, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var a models.Avatar

	query := `
			select * from avatar where user_id = $1
	`
	err := r.DB.QueryRowContext(ctx, query, userId).Scan(
		&a.PublicId,
		&a.Url,
		&a.UserId,
	)

	if err != nil {
		return a, err
	}

	return a, nil
}

// DeleteUsers deletes all users from the database.
// Returns an error if the operation fails.
func (r *AuthRepository) DeleteUsers() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `delete from users`
	_, err := r.DB.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	return nil
}

// DeleteAvatar deletes all avatars from the database.
// Returns an error if the operation fails.
func (r *AuthRepository) DeleteAvatar() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `delete from avatar`
	_, err := r.DB.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	return nil
}

// FetchUserByEmail fetches a user by email.
// Returns the user or an error.
func (r *AuthRepository) FetchUserByEmail(email string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user models.User

	query := `
		select user_id, name, email, password, role, created_at
		from users
		where email = $1
	`

	err := r.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.CreatedAt,
	)

	if err != nil {
		return &user, err
	}

	return &user, nil
}

// InsertToken inserts a token for a user, deleting any existing tokens for that user.
// Returns an error if the operation fails.
func (r *AuthRepository) InsertToken(t *models.Token, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// delete existing tokens
	query := `delete from tokens where user_id = $1`
	_, err := r.DB.ExecContext(ctx, query, userID)
	if err != nil {
		return err
	}

	query = `insert into tokens (token_hash, expiry, user_id, created_at, updated_at)
			values ($1, $2, $3, $4, $5)`

	_, err = r.DB.ExecContext(ctx, query,
		t.Hash,
		t.Expiry,
		userID,
		time.Now(),
		time.Now(),
	)

	if err != nil {
		return err
	}

	return nil
}

// FetchTokenById fetches a token by user ID.
// Returns the token or an error.
func (r *AuthRepository) FetchTokenById(id uuid.UUID) (*models.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var token models.Token

	query := `select * from tokens where user_id = $1`

	err := r.DB.QueryRowContext(ctx, query, id).Scan(
		&token.ID,
		&token.Hash,
		&token.Expiry,
		&token.UserID,
		&token.CreatedAt,
		&token.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &token, nil
}

// FetchUserByToken fetches a user by token string.
// Returns the user or an error.
func (r *AuthRepository) FetchUserByToken(token string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tokenHash := sha256.Sum256([]byte(token))
	var user models.User

	query := `
		select
			u.user_id, u.name, u.email, u.role
		from
			users u
			inner join tokens t on (u.user_id = t.user_id)
		where
			t.token_hash = $1
			and t.expiry > $2
	`

	err := r.DB.QueryRowContext(ctx, query, tokenHash[:], time.Now()).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Role,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// FetchUserById fetches a user by user ID.
// Returns the user or an error.
func (r *AuthRepository) FetchUserById(id uuid.UUID) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user models.User

	query := `select * from users where user_id = $1`

	err := r.DB.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.Role,
		&user.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// DeleteAvatarById deletes an avatar by its public ID.
// Returns an error if the operation fails.
func (r *AuthRepository) DeleteAvatarById(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `delete from avatar where public_id = $1`

	_, err := r.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

// FetchAllUsers returns all users in the database.
// Returns a slice of users or an error.
func (r *AuthRepository) FetchAllUsers() ([]*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var users []*models.User

	query := `select * from users`

	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.Password,
			&user.Role,
			&user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		users = append(users, &user)
	}

	return users, nil
}

// DeleteUserById deletes a user by user ID.
// Returns an error if the operation fails.
func (r *AuthRepository) DeleteUserById(id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `delete from users where user_id = $1`

	_, err := r.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

// DeleteTokenById deletes a token by user ID.
// Returns an error if the operation fails.
func (r *AuthRepository) DeleteTokenById(userId uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `delete from tokens where user_id = $1`

	_, err := r.DB.ExecContext(ctx, query, userId)
	if err != nil {
		return err
	}

	return nil
}
