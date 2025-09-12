// Package repository_test contains unit tests for the AuthRepository.
//
// These tests use sqlmock to simulate database interactions and comprehensively verify all repository methods, including user CRUD, avatar CRUD, token management, and user lookups by email, ID, or token.
//
// Each test covers both success and error cases, using table-driven and subtest patterns for maintainability and coverage. The newTestRepo helper ensures isolated, DRY test setup for each test case.
//
// For implementation details, see internal/auth/repository/repository.go.
package repository_test

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/auth/repository"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRepo(t *testing.T) (*repository.AuthRepository, sqlmock.Sqlmock, *sql.DB) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return repository.NewAuthRepository(db), mock, db
}

// TestAuthRepository_InsertUser verifies that a new user can be inserted into the database.
// It covers both the success case, where the user is inserted without issues,
// and the error case, where the database returns an error.
func TestAuthRepository_InsertUser(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	user := models.User{Name: "Test User", Email: "test@example.com", Password: "password", Role: "admin"}
	query := regexp.QuoteMeta(`insert into users (name, email, password, role, created_at) values ($1, $2, $3, $4, $5) returning user_id, name, email, password, role, created_at`)
	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"user_id", "name", "email", "password", "role", "created_at"}).
			AddRow(uuid.New(), user.Name, user.Email, user.Password, user.Role, time.Now())
		mock.ExpectQuery(query).
			WithArgs(user.Name, user.Email, user.Password, user.Role, sqlmock.AnyArg()).
			WillReturnRows(rows)
		result, err := repo.InsertUser(user)
		require.NoError(t, err)
		assert.Equal(t, user.Name, result.Name)
		assert.Equal(t, user.Email, result.Email)
		assert.Equal(t, user.Password, result.Password)
		assert.Equal(t, user.Role, result.Role)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(query).
			WithArgs(user.Name, user.Email, user.Password, user.Role, sqlmock.AnyArg()).
			WillReturnError(errors.New("db error"))
		_, err := repo.InsertUser(user)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_UpdateUser verifies updating a user's information, covering both success and database error cases.
func TestAuthRepository_UpdateUser(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	u := models.User{ID: uuid.New(), Name: "Test User", Email: "user@example.com", Password: "verySecret", Role: "admin"}
	query := regexp.QuoteMeta(`update users set name = $1, email = $2, password = $3, role = $4 where user_id = $5`)
	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(u.Name, u.Email, u.Password, u.Role, u.ID).WillReturnResult(sqlmock.NewResult(1, 1))
		err := repo.UpdateUser(u)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("exec error", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(u.Name, u.Email, u.Password, u.Role, u.ID).WillReturnError(errors.New("update error"))
		err := repo.UpdateUser(u)
		assert.Error(t, err)
		assert.Equal(t, "update error", err.Error())
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_InsertAvatar verifies inserting a new avatar for a user, covering both success and database error cases.
func TestAuthRepository_InsertAvatar(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	mockAvatar := &models.Avatar{PublicId: "testPublicId", Url: "testUrl", UserId: uuid.New()}
	query := regexp.QuoteMeta(`insert into avatar (public_id, url, user_id) values ($1, $2, $3) returning public_id, url, user_id`)
	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"public_id", "url", "user_id"}).AddRow(mockAvatar.PublicId, mockAvatar.Url, mockAvatar.UserId)
		mock.ExpectQuery(query).WithArgs(mockAvatar.PublicId, mockAvatar.Url, mockAvatar.UserId).WillReturnRows(rows)
		avatar, err := repo.InsertAvatar(mockAvatar)
		assert.NoError(t, err)
		assert.Equal(t, mockAvatar.PublicId, avatar.PublicId)
		assert.Equal(t, mockAvatar.Url, avatar.Url)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("db error", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(mockAvatar.PublicId, mockAvatar.Url, mockAvatar.UserId).WillReturnError(errors.New("insert error"))
		_, err := repo.InsertAvatar(mockAvatar)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_FetchAvatarById verifies fetching an avatar by user ID, covering both success and not found cases.
func TestAuthRepository_FetchAvatarById(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	userId := uuid.New()
	query := regexp.QuoteMeta(`select * from avatar where user_id = $1`)
	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"public_id", "url", "user_id"}).AddRow("pid", "url", userId)
		mock.ExpectQuery(query).WithArgs(userId).WillReturnRows(rows)
		avatar, err := repo.FetchAvatarById(userId)
		assert.NoError(t, err)
		assert.Equal(t, userId, avatar.UserId)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(userId).WillReturnError(sql.ErrNoRows)
		_, err := repo.FetchAvatarById(userId)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_DeleteUsers verifies deleting all users from the database, covering both success and error cases.
func TestAuthRepository_DeleteUsers(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	query := regexp.QuoteMeta(`delete from users`)
	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(1, 1))
		err := repo.DeleteUsers()
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("exec error", func(t *testing.T) {
		mock.ExpectExec(query).WillReturnError(errors.New("delete error"))
		err := repo.DeleteUsers()
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_DeleteAvatar verifies deleting all avatars from the database, covering both success and error cases.
func TestAuthRepository_DeleteAvatar(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	query := regexp.QuoteMeta(`delete from avatar`)
	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(0, 1))
		err := repo.DeleteAvatar()
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("exec error", func(t *testing.T) {
		mock.ExpectExec(query).WillReturnError(errors.New("delete error"))
		err := repo.DeleteAvatar()
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_FetchUserByEmail verifies fetching a user by email, covering both success and not found cases.
func TestAuthRepository_FetchUserByEmail(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	email := "test@example.com"
	user := models.User{ID: uuid.New(), Name: "Test User", Email: email, Password: "password", Role: "admin", CreatedAt: time.Now()}
	query := regexp.QuoteMeta(`select user_id, name, email, password, role, created_at from users where email = $1`)
	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"user_id", "name", "email", "password", "role", "created_at"}).
			AddRow(user.ID, user.Name, user.Email, user.Password, user.Role, user.CreatedAt)
		mock.ExpectQuery(query).WithArgs(email).WillReturnRows(rows)
		result, err := repo.FetchUserByEmail(email)
		assert.NoError(t, err)
		assert.Equal(t, user.Email, result.Email)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(email).WillReturnError(sql.ErrNoRows)
		_, err := repo.FetchUserByEmail(email)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_InsertToken verifies inserting a token for a user, covering success, delete error, and insert error cases.
func TestAuthRepository_InsertToken(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	token := &models.Token{Hash: []byte("hash"), Expiry: time.Now().Add(time.Hour)}
	userID := uuid.New()
	queryDelete := regexp.QuoteMeta(`delete from tokens where user_id = $1`)
	queryInsert := regexp.QuoteMeta(`insert into tokens (token_hash, expiry, user_id, created_at, updated_at) values ($1, $2, $3, $4, $5)`)
	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(queryDelete).WithArgs(userID).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(queryInsert).WithArgs(token.Hash, token.Expiry, userID, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
		err := repo.InsertToken(token, userID)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("delete error", func(t *testing.T) {
		mock.ExpectExec(queryDelete).WithArgs(userID).WillReturnError(errors.New("delete error"))
		err := repo.InsertToken(token, userID)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("insert error", func(t *testing.T) {
		mock.ExpectExec(queryDelete).WithArgs(userID).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(queryInsert).WithArgs(token.Hash, token.Expiry, userID, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(errors.New("insert error"))
		err := repo.InsertToken(token, userID)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_FetchTokenById verifies fetching a token by user ID, covering both success and not found cases.
func TestAuthRepository_FetchTokenById(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	id := uuid.New()
	query := regexp.QuoteMeta(`select * from tokens where user_id = $1`)
	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "token_hash", "expiry", "user_id", "created_at", "updated_at"}).
			AddRow(uuid.New(), []byte("hash"), time.Now().Add(time.Hour), id, time.Now(), time.Now())
		mock.ExpectQuery(query).WithArgs(id).WillReturnRows(rows)
		tok, err := repo.FetchTokenById(id)
		assert.NoError(t, err)
		assert.NotNil(t, tok)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(id).WillReturnError(sql.ErrNoRows)
		_, err := repo.FetchTokenById(id)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_FetchUserByToken verifies fetching a user by token, covering both success and not found cases.
func TestAuthRepository_FetchUserByToken(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	token := "sometoken"
	hash := sha256.Sum256([]byte(token))
	query := regexp.QuoteMeta(`select
			u.user_id, u.name, u.email, u.role
		from
			users u
			inner join tokens t on (u.user_id = t.user_id)
		where
			t.token_hash = $1
			and t.expiry > $2`)
	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"user_id", "name", "email", "role"}).AddRow(uuid.New(), "User", "user@example.com", "admin")
		mock.ExpectQuery(query).WithArgs(hash[:], sqlmock.AnyArg()).WillReturnRows(rows)
		user, err := repo.FetchUserByToken(token)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(hash[:], sqlmock.AnyArg()).WillReturnError(sql.ErrNoRows)
		_, err := repo.FetchUserByToken(token)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_FetchUserById verifies fetching a user by user ID, covering both success and not found cases.
func TestAuthRepository_FetchUserById(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	id := uuid.New()
	query := regexp.QuoteMeta(`select * from users where user_id = $1`)
	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"user_id", "name", "email", "password", "role", "created_at"}).
			AddRow(id, "User", "user@example.com", "password", "admin", time.Now())
		mock.ExpectQuery(query).WithArgs(id).WillReturnRows(rows)
		user, err := repo.FetchUserById(id)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, id, user.ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(query).WithArgs(id).WillReturnError(sql.ErrNoRows)
		_, err := repo.FetchUserById(id)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_DeleteAvatarById verifies deleting an avatar by public ID, covering both success and error cases.
func TestAuthRepository_DeleteAvatarById(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	id := "uniqueId"
	query := regexp.QuoteMeta(`delete from avatar where public_id = $1`)
	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(id).WillReturnResult(sqlmock.NewResult(1, 1))
		err := repo.DeleteAvatarById(id)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("exec error", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(id).WillReturnError(errors.New("delete error"))
		err := repo.DeleteAvatarById(id)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_FetchAllUsers verifies fetching all users from the database, covering success, query error, and scan error cases.
func TestAuthRepository_FetchAllUsers(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	query := regexp.QuoteMeta(`select * from users`)
	t.Run("success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"user_id", "name", "email", "password", "role", "created_at"}).
			AddRow(uuid.New(), "User1", "user1@example.com", "password1", "admin", time.Now()).
			AddRow(uuid.New(), "User2", "user2@example.com", "password2", "user", time.Now())
		mock.ExpectQuery(query).WillReturnRows(rows)
		users, err := repo.FetchAllUsers()
		assert.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "User1", users[0].Name)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("query error", func(t *testing.T) {
		mock.ExpectQuery(query).WillReturnError(errors.New("query error"))
		_, err := repo.FetchAllUsers()
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	// Scan error
	t.Run("scan error", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"user_id", "name", "email", "password", "role", "created_at"}).
			AddRow("bad-uuid", "User1", "user1@example.com", "password1", "admin", time.Now())
		mock.ExpectQuery(query).WillReturnRows(rows)
		_, err := repo.FetchAllUsers()
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_DeleteUserById verifies deleting a user by user ID, covering both success and error cases.
func TestAuthRepository_DeleteUserById(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	id := uuid.New()
	query := regexp.QuoteMeta(`delete from users where user_id = $1`)
	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(id).WillReturnResult(sqlmock.NewResult(1, 1))
		err := repo.DeleteUserById(id)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("exec error", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(id).WillReturnError(errors.New("delete error"))
		err := repo.DeleteUserById(id)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestAuthRepository_DeleteTokenById verifies deleting a token by user ID, covering both success and error cases.
func TestAuthRepository_DeleteTokenById(t *testing.T) {
	repo, mock, db := newTestRepo(t)
	defer db.Close()
	id := uuid.New()
	query := regexp.QuoteMeta(`delete from tokens where user_id = $1`)
	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(id).WillReturnResult(sqlmock.NewResult(1, 1))
		err := repo.DeleteTokenById(id)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("exec error", func(t *testing.T) {
		mock.ExpectExec(query).WithArgs(id).WillReturnError(errors.New("delete error"))
		err := repo.DeleteTokenById(id)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
