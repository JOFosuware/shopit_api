// Package usecase_test contains unit tests for the authentication use case layer.
//
// These tests cover all success, error, and edge cases for each use case method, using table-driven and subtest patterns. Test helpers are provided for DRY, isolated setup of mocks and use case instances. All tests use testify for assertions and mocking, and ensure proper test isolation and maintainability.
//
// See usecase.go for implementation and GoDoc for documentation.
package usecase_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/cloudinary/cloudinary-go/api/uploader"
	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/auth"
	mockRepo "github.com/jofosuware/go/shopit/internal/auth/mocks"
	"github.com/jofosuware/go/shopit/internal/auth/usecase"
	"github.com/jofosuware/go/shopit/internal/models"
	mockBcrypt "github.com/jofosuware/go/shopit/pkg/bcrypt/mocks"
	mockCloudinary "github.com/jofosuware/go/shopit/pkg/cloudinary/mocks"
	mockMail "github.com/jofosuware/go/shopit/pkg/mailer/mocks"
	"github.com/jofosuware/go/shopit/pkg/token"
	mockToken "github.com/jofosuware/go/shopit/pkg/token/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	cryptobcrypt "golang.org/x/crypto/bcrypt"
)

// newTestAuthUC returns a new AuthUC instance and its mocked dependencies for testing.
func newTestAuthUC(t *testing.T) (*usecase.AuthUC, *mockCloudinary.CloudUploader, *mockRepo.Repo, *mockToken.Tokener, *mockBcrypt.Encryptor, *mockMail.Mailer) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockRepo.NewRepo(t)
	mToken := mockToken.NewTokener(t)
	mBcrypt := mockBcrypt.NewEncryptor(t)
	mail := mockMail.NewMailer(t)
	return usecase.NewAuthUC(cld, repo, mToken, mBcrypt, mail, "https://app.example.test", "ShopIT <noreply@example.test>"), cld, repo, mToken, mBcrypt, mail
}

// TestAuthUC_Register tests the Register use case for all success and error scenarios.
func TestAuthUC_Register(t *testing.T) {
	a, cld, repo, mToken, mBcrypt, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Name: "test", Email: "user@gmail.com", Password: "userPassword", Role: "user"}
		cld.On("UploadToCloud", "avatar", "test").Return(&uploader.UploadResult{PublicID: "pid", URL: "url"}, nil).Once()
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(&models.User{}, sql.ErrNoRows).Once()
		hashedPassword := "hashed-password"
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return([]byte(hashedPassword), nil).Once()
		persistedUser := u
		persistedUser.Password = hashedPassword
		expiry := time.Now().Add(24 * time.Hour)
		generatedToken := &models.Token{PlainText: "tok", Hash: []byte("token-hash"), Expiry: expiry}
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(generatedToken, nil).Once()
		avatar := models.Avatar{PublicId: "pid", Url: "url", UserId: u.ID}
		storedToken := &models.Token{PlainText: "tok", Hash: generatedToken.Hash, Expiry: generatedToken.Expiry, Scope: token.ScopeAuthentication}
		createdUser := persistedUser
		createdUser.Avatar = avatar
		repo.On("CreateUserAtomically", mock.Anything, persistedUser, models.Avatar{PublicId: "pid", Url: "url"}, storedToken).Return(&createdUser, nil).Once()
		res, err := a.Register(context.Background(), u, "test")
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.True(t, res.Success)
		assert.Equal(t, "tok", res.Token)
		assert.Equal(t, u.ID, res.User.ID)
		assert.Equal(t, "test", res.User.Name)
		assert.Equal(t, "user@gmail.com", res.User.Email)
		assert.Equal(t, avatar, res.User.Avatar)
	})

	t.Run("User already exists", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Name: "test", Email: "user@gmail.com", Password: "userPassword", Role: "user"}
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(&u, nil).Once()
		res, err := a.Register(context.Background(), u, "test")
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Error hashing password", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Name: "test", Email: "user@gmail.com", Password: "userPassword", Role: "user"}
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(&models.User{}, sql.ErrNoRows).Once()
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return(nil, errors.New("hash error")).Once()
		res, err := a.Register(context.Background(), u, "test")
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Duplicate email discovered during atomic insert", func(t *testing.T) {
		u := models.User{Name: "test", Email: "race@example.test", Password: "userPassword", Role: "user"}
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(nil, sql.ErrNoRows).Once()
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return([]byte("hashed-password"), nil).Once()
		cld.On("UploadToCloud", "avatar", "test").Return(&uploader.UploadResult{PublicID: "duplicate-avatar"}, nil).Once()
		generatedToken := &models.Token{PlainText: "tok", Hash: []byte("hash"), Expiry: time.Now().Add(time.Hour)}
		mToken.On("GenerateToken", mock.Anything, 24*time.Hour, token.ScopeAuthentication).Return(generatedToken, nil).Once()
		repo.On("CreateUserAtomically", mock.Anything, mock.MatchedBy(func(user models.User) bool {
			return user.Email == u.Email && user.Password == "hashed-password"
		}), mock.Anything, mock.Anything).Return(nil, auth.ErrUserAlreadyExists).Once()
		cld.On("Destroy", "duplicate-avatar").Return(&uploader.DestroyResult{}, nil).Once()

		response, err := a.Register(context.Background(), u, "test")
		require.Nil(t, response)
		assert.ErrorIs(t, err, auth.ErrUserAlreadyExists)
	})
}

// TestAuthUC_Login tests the Login use case for all success and error scenarios.
func TestAuthUC_Login(t *testing.T) {
	a, _, repo, mToken, mBcrypt, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Email: "user@gmail.com", Password: "userPassword"}
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(&u, nil).Once()
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(u.Password)).Return(nil).Once()
		mToken.On("GenerateToken", u.ID, 24*time.Hour, "authentication").Return(&models.Token{}, nil).Once()
		repo.On("InsertToken", mock.Anything, &models.Token{Scope: token.ScopeAuthentication}, u.ID).Return(nil).Once()
		repo.On("FetchAvatarById", mock.Anything, u.ID).Return(models.Avatar{}, nil).Once()
		res, err := a.Login(context.Background(), u.Email, u.Password)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, u.ID, res.User.ID)
		assert.Equal(t, u.Email, res.User.Email)
	})

	t.Run("Failed Login - User not found", func(t *testing.T) {
		repo.On("FetchUserByEmail", mock.Anything, "missing@example.test").Return(nil, sql.ErrNoRows).Once()
		ur, err := a.Login(context.Background(), "missing@example.test", "password")
		require.Nil(t, ur)
		assert.ErrorIs(t, err, auth.ErrUserNotFound)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Login succeeds when the optional avatar is missing", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Email: "no-avatar@example.test", Password: "password-hash"}
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(&u, nil).Once()
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte("password")).Return(nil).Once()
		repo.On("FetchAvatarById", mock.Anything, u.ID).Return(models.Avatar{}, sql.ErrNoRows).Once()
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(&models.Token{}, nil).Once()
		repo.On("InsertToken", mock.Anything, &models.Token{Scope: token.ScopeAuthentication}, u.ID).Return(nil).Once()

		res, err := a.Login(context.Background(), u.Email, "password")

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, models.Avatar{}, res.User.Avatar)
	})

	t.Run("Failed Login - Incorrect password", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Email: "user@gmail.com", Password: "userPassword"}
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(&u, nil).Once()
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(u.Password)).Return(cryptobcrypt.ErrMismatchedHashAndPassword).Once()
		ur, err := a.Login(context.Background(), u.Email, u.Password)
		require.Nil(t, ur)
		assert.ErrorIs(t, err, auth.ErrInvalidPassword)
		assert.ErrorIs(t, err, cryptobcrypt.ErrMismatchedHashAndPassword)
	})
}

func TestAuthUC_Authenticate(t *testing.T) {
	a, _, repo, _, _, _ := newTestAuthUC(t)
	user := &models.User{ID: uuid.New(), Role: "user"}

	t.Run("success", func(t *testing.T) {
		repo.On("FetchUserByTokenWithScope", mock.Anything, "valid-token", token.ScopeAuthentication).Return(user, nil).Once()
		result, err := a.Authenticate(context.Background(), "valid-token")
		require.NoError(t, err)
		assert.Same(t, user, result)
	})

	t.Run("empty token", func(t *testing.T) {
		result, err := a.Authenticate(context.Background(), "")
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
		assert.Nil(t, result)
	})

	t.Run("token not found", func(t *testing.T) {
		repo.On("FetchUserByTokenWithScope", mock.Anything, "missing-token", token.ScopeAuthentication).Return(nil, sql.ErrNoRows).Once()
		result, err := a.Authenticate(context.Background(), "missing-token")
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
		assert.Nil(t, result)
	})
}

// TestAuthUC_SendPasswordResetEmail tests SendPasswordResetEmail for all scenarios.
func TestAuthUC_SendPasswordResetEmail(t *testing.T) {
	a, _, repo, mToken, _, mail := newTestAuthUC(t)
	u := models.User{ID: uuid.New(), Email: "user@gmail.com", Password: "userPassword"}

	t.Run("Success", func(t *testing.T) {
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(&u, nil).Once()
		tok := &models.Token{PlainText: "tok"}
		mToken.On("GenerateToken", u.ID, 60*time.Minute, token.ScopePasswordReset).Return(tok, nil).Once()
		mail.On("SendMail", mock.Anything, u.Email, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("InsertToken", mock.Anything, tok, u.ID).Return(nil).Once()
		res, err := a.SendPasswordResetEmail(context.Background(), u.Email)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.NotContains(t, res.Message, u.Email)
		assert.Contains(t, res.Message, "If an account exists")
	})

	t.Run("unknown email returns the same accepted response without sending mail", func(t *testing.T) {
		repo.On("FetchUserByEmail", mock.Anything, "missing@example.test").Return(nil, sql.ErrNoRows).Once()

		res, err := a.SendPasswordResetEmail(context.Background(), "missing@example.test")

		require.NoError(t, err)
		require.NotNil(t, res)
		assert.True(t, res.Success)
		assert.Equal(t, "If an account exists for that email, password reset instructions will be sent.", res.Message)
	})

	t.Run("Failed to send email", func(t *testing.T) {
		repo.On("FetchUserByEmail", mock.Anything, u.Email).Return(&u, nil).Once()
		tok := &models.Token{PlainText: "tok"}
		mToken.On("GenerateToken", u.ID, 60*time.Minute, token.ScopePasswordReset).Return(tok, nil).Once()
		mail.On("SendMail", mock.Anything, u.Email, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("mail error")).Once()
		repo.On("InsertToken", mock.Anything, tok, u.ID).Return(nil).Once()
		res, err := a.SendPasswordResetEmail(context.Background(), u.Email)
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

// TestAuthUC_ResetPassword tests the ResetPassword use case for all success and error scenarios.
func TestAuthUC_ResetPassword(t *testing.T) {
	a, _, repo, mToken, mBcrypt, _ := newTestAuthUC(t)

	u := models.User{
		ID:       uuid.New(),
		Email:    "user@gmail.com",
		Password: "verySecret",
	}

	t.Run("Success", func(t *testing.T) {
		repo.On("FetchUserByTokenWithScope", mock.Anything, "token", token.ScopePasswordReset).Return(&u, nil).Once()
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return([]byte("verySecret"), nil).Once()
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(&models.Token{}, nil).Once()
		updatedUser := u
		updatedUser.Password = "verySecret"
		authToken := &models.Token{Scope: token.ScopeAuthentication}
		repo.On("UpdatePasswordAndReplaceToken", mock.Anything, updatedUser, authToken, true).Return(nil).Once()
		res, err := a.ResetPassword(context.Background(), "token", u.Password)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, u.ID, res.User.ID)
		assert.Equal(t, u.Email, res.User.Email)
	})

	t.Run("Failed Reset - User not found", func(t *testing.T) {
		repo.On("FetchUserByTokenWithScope", mock.Anything, "invalid_token", token.ScopePasswordReset).Return(nil, sql.ErrNoRows).Once()
		res, err := a.ResetPassword(context.Background(), "invalid_token", "newPassword")
		require.Nil(t, res)
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Failed Reset - Error updating user", func(t *testing.T) {
		repo.On("FetchUserByTokenWithScope", mock.Anything, "token", token.ScopePasswordReset).Return(&u, nil).Once()
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return([]byte("verySecret"), nil).Once()
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(&models.Token{}, nil).Once()
		updatedUser := u
		updatedUser.Password = "verySecret"
		authToken := &models.Token{Scope: token.ScopeAuthentication}
		repo.On("UpdatePasswordAndReplaceToken", mock.Anything, updatedUser, authToken, true).Return(errors.New("update error")).Once()
		res, err := a.ResetPassword(context.Background(), "token", u.Password)
		require.Nil(t, res)
		assert.ErrorIs(t, err, auth.ErrPasswordUpdateFailed)
	})
}

// TestUpdatedPassword tests the UpdatePassword use case for all success and error scenarios.
func TestUpdatedPassword(t *testing.T) {
	a, _, repo, mToken, mBcrypt, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		passwords := models.Passwords{
			Password:    "newPassword",
			OldPassword: "oldPassword",
		}
		u := models.User{
			ID:       uuid.New(),
			Password: "oldPassword",
		}
		repo.On("FetchUserById", mock.Anything, u.ID).Return(&u, nil)
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(passwords.OldPassword)).Return(nil)
		mBcrypt.On("GenerateFromPassword", []byte(passwords.Password)).Return([]byte(passwords.Password), nil)
		authToken := &models.Token{Scope: token.ScopeAuthentication}
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(authToken, nil)
		updatedUser := u
		updatedUser.Password = "newPassword"
		repo.On("UpdatePasswordAndReplaceToken", mock.Anything, updatedUser, authToken, false).Return(nil).Once()
		res, err := a.UpdatePassword(context.Background(), u.ID, passwords)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, u.ID, res.User.ID)
	})

	t.Run("Failed Update - User not found", func(t *testing.T) {
		passwords := models.Passwords{
			Password:    "newPassword",
			OldPassword: "oldPassword",
		}
		u := models.User{
			ID: uuid.New(),
		}
		repo.On("FetchUserById", mock.Anything, u.ID).Return(nil, sql.ErrNoRows).Once()
		res, err := a.UpdatePassword(context.Background(), u.ID, passwords)
		require.Nil(t, res)
		assert.ErrorIs(t, err, auth.ErrUserNotFound)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Failed Update - Incorrect old password", func(t *testing.T) {
		passwords := models.Passwords{
			Password:    "newPassword",
			OldPassword: "wrongOldPassword",
		}
		u := models.User{
			ID:       uuid.New(),
			Password: "oldPassword",
		}
		repo.On("FetchUserById", mock.Anything, u.ID).Return(&u, nil)
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(passwords.OldPassword)).Return(cryptobcrypt.ErrMismatchedHashAndPassword)
		res, err := a.UpdatePassword(context.Background(), u.ID, passwords)
		require.Nil(t, res)
		assert.ErrorIs(t, err, auth.ErrInvalidOldPassword)
		assert.ErrorIs(t, err, cryptobcrypt.ErrMismatchedHashAndPassword)
	})

	t.Run("Failed Update - Error updating user", func(t *testing.T) {
		passwords := models.Passwords{
			Password:    "newPassword",
			OldPassword: "oldPassword",
		}
		u := models.User{
			ID:       uuid.New(),
			Password: "oldPassword",
		}
		repo.On("FetchUserById", mock.Anything, u.ID).Return(&u, nil)
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(passwords.OldPassword)).Return(nil)
		mBcrypt.On("GenerateFromPassword", []byte(passwords.Password)).Return([]byte(passwords.Password), nil)
		authToken := &models.Token{Scope: token.ScopeAuthentication}
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(authToken, nil)
		updatedUser := u
		updatedUser.Password = "newPassword"
		repo.On("UpdatePasswordAndReplaceToken", mock.Anything, updatedUser, authToken, false).Return(errors.New("update error")).Once()
		res, err := a.UpdatePassword(context.Background(), u.ID, passwords)
		require.Nil(t, res)
		assert.ErrorIs(t, err, auth.ErrPasswordUpdateFailed)
	})
}

// TestUpdateProfile tests the UpdateProfile use case for all success and error scenarios.
func TestUpdateProfile(t *testing.T) {
	a, cld, repo, _, _, _ := newTestAuthUC(t)

	u := models.User{
		ID:       uuid.New(),
		Email:    "user@gmail.com",
		Password: "verySecret",
		Name:     "John Doe",
	}
	avatar := models.Avatar{
		PublicId: "publicId",
		Url:      "url",
		UserId:   u.ID,
	}

	t.Run("Success", func(t *testing.T) {
		res := uploader.UploadResult{
			PublicID: "new-public-id",
			URL:      "new-url",
		}
		repo.On("FetchAvatarById", mock.Anything, u.ID).Return(avatar, nil).Once()
		cld.On("UploadToCloud", "avatar", "user.jpg").Return(&res, nil).Once()
		newAvatar := models.Avatar{PublicId: res.PublicID, Url: res.URL, UserId: u.ID}
		repo.On("UpdateProfile", mock.Anything, u.ID, u.Name, u.Email, &newAvatar).Return(nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, nil).Once()
		err := a.UpdateProfile(context.Background(), u, "user.jpg")
		assert.NoError(t, err)
	})

	t.Run("Failed Update - User not found", func(t *testing.T) {
		repo.On("FetchAvatarById", mock.Anything, u.ID).Return(models.Avatar{}, errors.New("user not found")).Once()
		err := a.UpdateProfile(context.Background(), u, "user.jpg")
		assert.Error(t, err)
	})

	t.Run("Failed Update - Error deleting old avatar", func(t *testing.T) {
		repo.On("FetchAvatarById", mock.Anything, u.ID).Return(avatar, nil).Once()
		upload := &uploader.UploadResult{PublicID: "new-public-id", URL: "new-url"}
		cld.On("UploadToCloud", "avatar", "user.jpg").Return(upload, nil).Once()
		repo.On("UpdateProfile", mock.Anything, u.ID, u.Name, u.Email, mock.Anything).Return(nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, errors.New("cloudinary error")).Once()
		err := a.UpdateProfile(context.Background(), u, "user.jpg")
		assert.ErrorIs(t, err, auth.ErrAvatarDeletionFailed)
	})

	t.Run("Failed Update - Error uploading new avatar", func(t *testing.T) {
		u := models.User{
			ID:       uuid.New(),
			Email:    "user@gmail.com",
			Password: "verySecret",
			Name:     "John Doe",
		}
		avatar := models.Avatar{
			PublicId: "publicId",
			Url:      "url",
			UserId:   u.ID,
		}
		res := uploader.UploadResult{
			PublicID: "publicId",
			URL:      "url",
		}
		repo.On("FetchAvatarById", mock.Anything, u.ID).Return(avatar, nil).Once()
		cld.On("UploadToCloud", "avatar", "user.jpg").Return(&res, errors.New("upload error")).Once()
		err := a.UpdateProfile(context.Background(), u, "user.jpg")
		assert.Error(t, err)
	})

	t.Run("Failed Update - Error inserting new avatar", func(t *testing.T) {
		u := models.User{
			ID:       uuid.New(),
			Email:    "user@gmail.com",
			Password: "verySecret",
			Name:     "John Doe",
		}
		avatar := models.Avatar{
			PublicId: "publicId",
			Url:      "url",
			UserId:   u.ID,
		}
		res := uploader.UploadResult{
			PublicID: "publicId",
			URL:      "url",
		}
		repo.On("FetchAvatarById", mock.Anything, u.ID).Return(avatar, nil).Once()
		cld.On("UploadToCloud", "avatar", "user.jpg").Return(&res, nil).Once()
		repo.On("UpdateProfile", mock.Anything, u.ID, u.Name, u.Email, mock.Anything).Return(errors.New("update error")).Once()
		cld.On("Destroy", res.PublicID).Return(&uploader.DestroyResult{}, nil).Once()
		err := a.UpdateProfile(context.Background(), u, "user.jpg")
		assert.Error(t, err)
	})

	t.Run("Failed to fetch current avatar", func(t *testing.T) {
		newUser := models.User{ID: uuid.New(), Name: "Jane", Email: "jane@example.test"}
		repo.On("FetchAvatarById", mock.Anything, newUser.ID).Return(models.Avatar{}, sql.ErrNoRows).Once()
		upload := &uploader.UploadResult{PublicID: "first-avatar", URL: "first-url"}
		cld.On("UploadToCloud", "avatar", "user.jpg").Return(upload, nil).Once()
		newAvatar := models.Avatar{PublicId: upload.PublicID, Url: upload.URL, UserId: newUser.ID}
		repo.On("UpdateProfile", mock.Anything, newUser.ID, newUser.Name, newUser.Email, &newAvatar).Return(nil).Once()
		require.NoError(t, a.UpdateProfile(context.Background(), newUser, "user.jpg"))
	})
}

// TestGetAllUsers tests the GetAllUsers use case for all success and error scenarios.
func TestGetAllUsers(t *testing.T) {
	a, _, repo, _, _, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		user := &models.UserPublic{
			ID:     uuid.New(),
			Name:   "Test User",
			Email:  "user@example.test",
			Role:   "user",
			Avatar: models.Avatar{PublicId: "avatar-id", Url: "avatar-url"},
		}
		repo.On("FetchAllUsers", mock.Anything).Return([]*models.UserPublic{user}, nil).Once()
		users, err := a.GetAllUsers(context.Background(), &models.User{Role: "admin"})
		assert.NoError(t, err)
		assert.NotNil(t, users)
		require.Len(t, users, 1)
		assert.Equal(t, models.UserPublic{
			ID:     user.ID,
			Name:   user.Name,
			Email:  user.Email,
			Role:   user.Role,
			Avatar: user.Avatar,
		}, *users[0])
	})

	t.Run("Failed to fetch users", func(t *testing.T) {
		repo.On("FetchAllUsers", mock.Anything).Return(nil, errors.New("fetch error")).Once()
		users, err := a.GetAllUsers(context.Background(), &models.User{Role: "admin"})
		assert.Error(t, err)
		assert.Nil(t, users)
	})
}

// TestGetUserDetails tests the GetUserDetails use case for all success and error scenarios.
func TestGetUserDetails(t *testing.T) {
	a, _, repo, _, _, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		id := uuid.New()
		storedUser := &models.User{
			ID:       id,
			Name:     "Test User",
			Email:    "user@example.test",
			Password: "sensitive-password",
			Role:     "user",
		}
		avatar := models.Avatar{PublicId: "avatar-id", Url: "avatar-url", UserId: id}
		repo.On("FetchUserById", mock.Anything, id).Return(storedUser, nil)
		repo.On("FetchAvatarById", mock.Anything, id).Return(avatar, nil)
		details, err := a.GetUserDetails(context.Background(), &models.User{Role: "admin"}, id)
		assert.NoError(t, err)
		assert.NotNil(t, details)
		assert.Equal(t, id, details.ID)
		assert.Equal(t, "Test User", details.Name)
		assert.Equal(t, "user@example.test", details.Email)
		assert.Equal(t, avatar, details.Avatar)
	})

	t.Run("Failed - User not found", func(t *testing.T) {
		id := uuid.New()
		repo.On("FetchUserById", mock.Anything, id).Return(nil, sql.ErrNoRows).Once()
		user, err := a.GetUserDetails(context.Background(), &models.User{Role: "admin"}, id)
		require.Nil(t, user)
		assert.ErrorIs(t, err, auth.ErrUserNotFound)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Failed - Error fetching avatar", func(t *testing.T) {
		id := uuid.New()
		repo.On("FetchUserById", mock.Anything, id).Return(&models.User{}, nil)
		repo.On("FetchAvatarById", mock.Anything, id).Return(models.Avatar{}, errors.New("avatar not found"))
		user, err := a.GetUserDetails(context.Background(), &models.User{Role: "admin"}, id)
		assert.Error(t, err)
		assert.Nil(t, user)
	})
}

// TestUpdateUser tests the UpdateUser use case for all success and error scenarios.
func TestUpdateUser(t *testing.T) {
	a, _, repo, _, _, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		id := uuid.New()
		u := models.User{
			ID:       id,
			Email:    "user@gmail.com",
			Password: "verySecret",
			Name:     "John Doe",
		}
		repo.On("FetchUserById", mock.Anything, id).Return(&u, nil)
		repo.On("UpdateUser", mock.Anything, u).Return(nil)
		res, err := a.UpdateUser(context.Background(), &models.User{Role: "admin"}, id, u)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, id, res.User.ID)
		assert.Equal(t, u.Name, res.User.Name)
		assert.Equal(t, u.Email, res.User.Email)
	})

	t.Run("Failed Update - User not found", func(t *testing.T) {
		id := uuid.New()
		u := models.User{
			ID:       id,
			Email:    "user@gmail.com",
			Password: "verySecret",
			Name:     "John Doe",
		}
		repo.On("FetchUserById", mock.Anything, id).Return(nil, sql.ErrNoRows).Once()
		res, err := a.UpdateUser(context.Background(), &models.User{Role: "admin"}, id, u)
		require.Nil(t, res)
		assert.ErrorIs(t, err, auth.ErrUserNotFound)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Failed Update - Error updating user", func(t *testing.T) {
		id := uuid.New()
		u := models.User{
			ID:       id,
			Email:    "user@gmail.com",
			Password: "verySecret",
			Name:     "John Doe",
		}
		repo.On("FetchUserById", mock.Anything, id).Return(&u, nil)
		repo.On("UpdateUser", mock.Anything, u).Return(errors.New("update error"))
		res, err := a.UpdateUser(context.Background(), &models.User{Role: "admin"}, id, u)
		require.Nil(t, res)
		assert.ErrorIs(t, err, auth.ErrUserUpdateFailed)
	})
}

// TestDeleteUser tests the DeleteUser use case for all success and error scenarios.
func TestDeleteUser(t *testing.T) {
	a, cld, repo, _, _, _ := newTestAuthUC(t)

	id := uuid.New()
	avatar := models.Avatar{
		PublicId: "publicId",
		Url:      "url",
		UserId:   id,
	}

	t.Run("Success", func(t *testing.T) {
		repo.On("FetchAvatarById", mock.Anything, id).Return(avatar, nil).Once()
		repo.On("DeleteUserById", mock.Anything, id).Return(nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, nil).Once()
		err := a.DeleteUser(context.Background(), &models.User{Role: "admin"}, id)
		assert.NoError(t, err)
	})

	t.Run("Failed Delete - User not found", func(t *testing.T) {
		repo.On("FetchAvatarById", mock.Anything, id).Return(models.Avatar{}, sql.ErrNoRows).Once()
		repo.On("DeleteUserById", mock.Anything, id).Return(sql.ErrNoRows).Once()
		err := a.DeleteUser(context.Background(), &models.User{Role: "admin"}, id)
		assert.ErrorIs(t, err, auth.ErrUserNotFound)
	})

	t.Run("Failed Delete - Avatar cleanup error after deleting user", func(t *testing.T) {
		repo.On("FetchAvatarById", mock.Anything, id).Return(avatar, nil).Once()
		repo.On("DeleteUserById", mock.Anything, id).Return(nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(nil, errors.New("cloudinary error")).Once()
		err := a.DeleteUser(context.Background(), &models.User{Role: "admin"}, id)
		assert.ErrorIs(t, err, auth.ErrAvatarDeletionFailed)
	})

	t.Run("Failed Delete - Error deleting user", func(t *testing.T) {
		repo.On("FetchAvatarById", mock.Anything, id).Return(avatar, nil).Once()
		repo.On("DeleteUserById", mock.Anything, id).Return(errors.New("delete error")).Once()
		err := a.DeleteUser(context.Background(), &models.User{Role: "admin"}, id)
		assert.ErrorIs(t, err, auth.ErrUserDeletionFailed)
	})
}

// TestLogout tests the DeleteUserToken use case for all success and error scenarios.
func TestLogout(t *testing.T) {
	a, _, repo, _, _, _ := newTestAuthUC(t)
	t.Run("Success", func(t *testing.T) {
		tok := "MQUYLLXB2PHU5PE6PG3HGG2AXI"
		repo.On("DeleteAuthenticationToken", mock.Anything, tok).Return(nil).Once()
		err := a.DeleteUserToken(context.Background(), tok)
		assert.NoError(t, err)
	})

	t.Run("Failed Logout - Token not found", func(t *testing.T) {
		tok := "INVALIDTOKEN"
		repo.On("DeleteAuthenticationToken", mock.Anything, tok).Return(sql.ErrNoRows).Once()
		err := a.DeleteUserToken(context.Background(), tok)
		assert.ErrorIs(t, err, auth.ErrTokenNotFound)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Failed Logout - Error deleting token", func(t *testing.T) {
		tok := "MQUYLLXB2PHU5PE6PG3HGG2AXI"
		repo.On("DeleteAuthenticationToken", mock.Anything, tok).Return(errors.New("delete error")).Once()
		err := a.DeleteUserToken(context.Background(), tok)
		assert.Error(t, err)
	})
}
