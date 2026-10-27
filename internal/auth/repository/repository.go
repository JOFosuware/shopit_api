// internal/auth/repository/repository.go
// Package repository provides the data access layer for authentication-related operations.
// It implements methods for user, avatar, and token management using a SQL database.
package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jofosuware/go/shopit/internal/auth"
	"github.com/jofosuware/go/shopit/internal/models"
	pkgtoken "github.com/jofosuware/go/shopit/pkg/token"
)

// AuthRepository provides methods for interacting with the authentication-related tables in the database.
// It should be constructed with a *sql.DB connection.
type AuthRepository struct {
	// DB is the database connection.
	DB *sql.DB
}

// NewAuthRepository returns a new AuthRepository for the provided database.
func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{
		DB: db,
	}
}

func mapEmailUniquenessError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
		return fmt.Errorf("%w: %w", auth.ErrUserAlreadyExists, err)
	}
	return err
}

// InsertUser inserts a new user into the users table.
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
func (r *AuthRepository) UpdateUser(ctx context.Context, u models.User) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `update users set name = $1, email = $2, password = $3, role = $4 where user_id = $5`

	result, err := r.DB.ExecContext(ctx, query,
		u.Name,
		u.Email,
		u.Password,
		u.Role,
		u.ID,
	)

	if err != nil {
		return mapEmailUniquenessError(err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdatePasswordAndReplaceToken atomically persists a password change and rotates
// authentication credentials. When revokeResetTokens is true, all reset tokens for
// the user are consumed in the same transaction.
func (r *AuthRepository) UpdatePasswordAndReplaceToken(ctx context.Context, u models.User, t *models.Token, revokeResetTokens bool) error {
	if t == nil {
		return errors.New("token must not be nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const updateUserQuery = `update users set name = $1, email = $2, password = $3, role = $4 where user_id = $5`
	result, err := tx.ExecContext(ctx, updateUserQuery, u.Name, u.Email, u.Password, u.Role, u.ID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if _, err = tx.ExecContext(ctx, `delete from tokens where user_id = $1 and scope = $2`, u.ID, pkgtoken.ScopeAuthentication); err != nil {
		return err
	}
	if revokeResetTokens {
		if _, err = tx.ExecContext(ctx, `delete from tokens where user_id = $1 and scope = $2`, u.ID, pkgtoken.ScopePasswordReset); err != nil {
			return err
		}
	}

	now := time.Now()
	const insertTokenQuery = `insert into tokens (token_hash, expiry, user_id, scope, created_at, updated_at) values ($1, $2, $3, $4, $5, $6)`
	if _, err = tx.ExecContext(ctx, insertTokenQuery, t.Hash, t.Expiry, u.ID, t.Scope, now, now); err != nil {
		return err
	}

	return tx.Commit()
}

// InsertAvatar inserts a new avatar record for a user.
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
		a.PublicId,
		a.Url,
		a.UserId,
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
func (r *AuthRepository) FetchAvatarById(ctx context.Context, userId uuid.UUID) (models.Avatar, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var a models.Avatar

	query := `
			select public_id, url, user_id from avatar where user_id = $1
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
func (r *AuthRepository) FetchUserByEmail(ctx context.Context, email string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
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
		return nil, err
	}

	return &user, nil
}

// InsertToken inserts a token for a user, deleting any existing tokens for that user.
func (r *AuthRepository) InsertToken(ctx context.Context, t *models.Token, userID uuid.UUID) error {
	if t == nil {
		return errors.New("token must not be nil")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// delete existing tokens of the same scope for this user (allow auth and reset tokens to coexist)
	query := `delete from tokens where user_id = $1 and scope = $2`
	_, err = tx.ExecContext(ctx, query, userID, t.Scope)
	if err != nil {
		return err
	}

	query = `insert into tokens (token_hash, expiry, user_id, scope, created_at, updated_at)
			values ($1, $2, $3, $4, $5, $6)`

	_, err = tx.ExecContext(ctx, query,
		t.Hash,
		t.Expiry,
		userID,
		t.Scope,
		time.Now(),
		time.Now(),
	)

	if err != nil {
		return err
	}

	return tx.Commit()
}

// FetchTokenById fetches a token by user ID.
func (r *AuthRepository) FetchTokenById(id uuid.UUID) (*models.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var token models.Token

	// select fields explicitly including scope
	query := `select token_id, token_hash, expiry, user_id, scope, created_at, updated_at from tokens where user_id = $1`

	err := r.DB.QueryRowContext(ctx, query, id).Scan(
		&token.ID,
		&token.Hash,
		&token.Expiry,
		&token.UserID,
		&token.Scope,
		&token.CreatedAt,
		&token.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &token, nil
}

// FetchUserByToken fetches a user by token string.
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
func (r *AuthRepository) FetchUserById(ctx context.Context, id uuid.UUID) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var user models.User

	query := `select user_id, name, email, password, role, created_at from users where user_id = $1`

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

// FetchUserByTokenWithScope fetches a user by token string and required scope.
func (r *AuthRepository) FetchUserByTokenWithScope(ctx context.Context, token string, scope string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
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
			and t.scope = $3
	`

	err := r.DB.QueryRowContext(ctx, query, tokenHash[:], time.Now(), scope).Scan(
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

// DeleteAvatarById deletes an avatar by its public ID.
func (r *AuthRepository) DeleteAvatarById(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `delete from avatar where public_id = $1`

	_, err := r.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}

// FetchAllUsers returns all users in the database.
func (r *AuthRepository) FetchAllUsers(ctx context.Context) ([]*models.UserPublic, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	users := make([]*models.UserPublic, 0)

	query := `select user_id, name, email, role, created_at from users`

	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user models.UserPublic
		err := rows.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.Role,
			&user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		users = append(users, &user)

	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// DeleteUserById deletes a user by user ID.
func (r *AuthRepository) DeleteUserById(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `delete from users where user_id = $1`

	result, err := r.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteTokenById deletes a token by user ID.
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

// DeleteAuthenticationToken removes only the presented authentication token.
func (r *AuthRepository) DeleteAuthenticationToken(ctx context.Context, plainTextToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tokenHash := sha256.Sum256([]byte(plainTextToken))
	result, err := r.DB.ExecContext(ctx,
		`delete from tokens where token_hash = $1 and scope = $2`,
		tokenHash[:],
		pkgtoken.ScopeAuthentication,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdateProfile updates profile fields and optionally replaces the avatar in one transaction.
func (r *AuthRepository) UpdateProfile(ctx context.Context, userID uuid.UUID, name, email string, avatar *models.Avatar) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `update users set name = $1, email = $2 where user_id = $3`, name, email, userID)
	if err != nil {
		return mapEmailUniquenessError(err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if avatar != nil {
		if _, err = tx.ExecContext(ctx, `delete from avatar where user_id = $1`, userID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx,
			`insert into avatar (public_id, url, user_id) values ($1, $2, $3)`,
			avatar.PublicId,
			avatar.Url,
			userID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CreateUserAtomically inserts User, Avatar, and Token within a single database transaction.
// This ensures that the database state is consistent across all necessary write operations.
func (r *AuthRepository) CreateUserAtomically(ctx context.Context, user models.User, avatar models.Avatar, t *models.Token) (*models.User, error) {
	if t == nil {
		return nil, errors.New("token must not be nil")
	}
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert User
	var insertedUser models.User
	userInsertQuery := `insert into users (user_id, name, email, password, role, created_at) values ($1, $2, $3, $4, $5, $6) returning user_id, name, email, password, role, created_at`
	err = tx.QueryRowContext(ctx, userInsertQuery,
		user.ID,
		user.Name,
		user.Email,
		user.Password,
		user.Role,
		time.Now(),
	).Scan(
		&insertedUser.ID,
		&insertedUser.Name,
		&insertedUser.Email,
		&insertedUser.Password,
		&insertedUser.Role,
		&insertedUser.CreatedAt,
	)
	if err != nil {
		if mappedErr := mapEmailUniquenessError(err); errors.Is(mappedErr, auth.ErrUserAlreadyExists) {
			return nil, mappedErr
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	// 2. Insert Avatar
	var insertedAvatar models.Avatar
	avatarInsertQuery := `
		insert into avatar 
			(public_id, url, user_id)
		values
			($1, $2, $3)
		returning public_id, url, user_id
	`
	err = tx.QueryRowContext(ctx, avatarInsertQuery,
		avatar.PublicId,
		avatar.Url,
		insertedUser.ID,
	).Scan(
		&insertedAvatar.PublicId,
		&insertedAvatar.Url,
		&insertedAvatar.UserId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert avatar: %w", err)
	}

	// 3. Insert Token
	now := time.Now()
	tokenInsertQuery := `insert into tokens (token_hash, expiry, user_id, scope, created_at, updated_at) values ($1, $2, $3, $4, $5, $6)`
	_, err = tx.ExecContext(ctx, tokenInsertQuery,
		t.Hash,
		t.Expiry,
		insertedUser.ID,
		t.Scope,
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert token: %w", err)
	}

	// Commit all changes together
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update the user struct to reflect the newly inserted avatar for return consistency
	insertedUser.Avatar = insertedAvatar
	return &insertedUser, nil
}
