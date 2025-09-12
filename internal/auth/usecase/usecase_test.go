// Package usecase_test contains unit tests for the authentication use case layer.
//
// These tests cover all success, error, and edge cases for each use case method, using table-driven and subtest patterns. Test helpers are provided for DRY, isolated setup of mocks and use case instances. All tests use testify for assertions and mocking, and ensure proper test isolation and maintainability.
//
// See usecase.go for implementation and GoDoc for documentation.
package usecase_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/cloudinary/cloudinary-go/api/uploader"
	"github.com/google/uuid"
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
)

// newTestAuthUC returns a new AuthUC instance and its mocked dependencies for testing.
func newTestAuthUC(t *testing.T) (*usecase.AuthUC, *mockCloudinary.CloudUploader, *mockRepo.Repo, *mockToken.Tokener, *mockBcrypt.Encryptor, *mockMail.Mailer) {
	cld := mockCloudinary.NewCloudUploader(t)
	repo := mockRepo.NewRepo(t)
	mToken := mockToken.NewTokener(t)
	mBcrypt := mockBcrypt.NewEncryptor(t)
	mail := mockMail.NewMailer(t)
	return usecase.NewAuthUC(cld, repo, mToken, mBcrypt, mail), cld, repo, mToken, mBcrypt, mail
}

// TestAuthUC_Register tests the Register use case for all success and error scenarios.
func TestAuthUC_Register(t *testing.T) {
	a, cld, repo, mToken, mBcrypt, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Name: "test", Email: "user@gmail.com", Password: "userPassword", Role: "user"}
		cld.On("UploadToCloud", "avatar", "test").Return(&uploader.UploadResult{PublicID: "pid", URL: "url"}, nil)
		repo.On("FetchUserByEmail", u.Email).Return(&models.User{}, errors.New("sql: no rows in result set")).Once()
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return([]byte(u.Password), nil).Once()
		repo.On("InsertUser", u).Return(&u, nil).Once()
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(&models.Token{PlainText: "tok"}, nil).Once()
		repo.On("InsertToken", &models.Token{PlainText: "tok"}, u.ID).Return(nil).Once()
		repo.On("InsertAvatar", &models.Avatar{PublicId: "pid", Url: "url", UserId: u.ID}).Return(models.Avatar{PublicId: "pid", Url: "url", UserId: u.ID}, nil).Once()
		res, err := a.Register(u, "test")
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("User already exists", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Name: "test", Email: "user@gmail.com", Password: "userPassword", Role: "user"}
		repo.On("FetchUserByEmail", u.Email).Return(&u, nil).Once()
		res, err := a.Register(u, "test")
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Error hashing password", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Name: "test", Email: "user@gmail.com", Password: "userPassword", Role: "user"}
		repo.On("FetchUserByEmail", u.Email).Return(&models.User{}, errors.New("sql: no rows in result set")).Once()
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return(nil, errors.New("hash error")).Once()
		res, err := a.Register(u, "test")
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

// TestAuthUC_Login tests the Login use case for all success and error scenarios.
func TestAuthUC_Login(t *testing.T) {
	a, _, repo, mToken, mBcrypt, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Email: "user@gmail.com", Password: "userPassword"}
		repo.On("FetchUserByEmail", u.Email).Return(&u, nil).Once()
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(u.Password)).Return(nil).Once()
		mToken.On("GenerateToken", u.ID, 24*time.Hour, "authentication").Return(&models.Token{}, nil).Once()
		repo.On("InsertToken", &models.Token{}, u.ID).Return(nil).Once()
		repo.On("FetchAvatarById", u.ID).Return(models.Avatar{}, nil).Once()
		res, err := a.Login(u.Email, u.Password)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Failed Login - User not found", func(t *testing.T) {
		repo.On("FetchUserByEmail", "").Return(nil, errors.New("error"))
		ur, err := a.Login("", "")
		assert.Error(t, err)
		assert.Nil(t, ur)
	})

	t.Run("Failed Login - Incorrect password", func(t *testing.T) {
		u := models.User{ID: uuid.New(), Email: "user@gmail.com", Password: "userPassword"}
		repo.On("FetchUserByEmail", u.Email).Return(&u, nil).Once()
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(u.Password)).Return(errors.New("wrong password")).Once()
		ur, err := a.Login(u.Email, u.Password)
		assert.Error(t, err)
		assert.Nil(t, ur)
	})
}

// TestAuthUC_SendPasswordResetEmail tests SendPasswordResetEmail for all scenarios.
func TestAuthUC_SendPasswordResetEmail(t *testing.T) {
	a, _, repo, mToken, _, mail := newTestAuthUC(t)
	u := models.User{ID: uuid.New(), Email: "user@gmail.com", Password: "userPassword"}

	t.Run("Success", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/forget-password", nil)
		require.NoError(t, err)
		repo.On("FetchUserByEmail", u.Email).Return(&u, nil).Once()
		tok := &models.Token{PlainText: "tok"}
		mToken.On("GenerateToken", u.ID, 60*time.Minute, token.ScopeAuthentication).Return(tok, nil).Once()
		mail.On("SendMail", mock.Anything, u.Email, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("InsertToken", tok, u.ID).Return(nil).Once()
		res, err := a.SendPasswordResetEmail(u.Email, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Failed to send email", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/forget-password", nil)
		require.NoError(t, err)
		repo.On("FetchUserByEmail", u.Email).Return(&u, nil).Once()
		tok := &models.Token{PlainText: "tok"}
		mToken.On("GenerateToken", u.ID, 60*time.Minute, token.ScopeAuthentication).Return(tok, nil).Once()
		mail.On("SendMail", mock.Anything, u.Email, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("mail error")).Once()
		res, err := a.SendPasswordResetEmail(u.Email, req)
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
		repo.On("FetchUserByToken", "token").Return(&u, nil).Once()
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return([]byte("verySecret"), nil).Once()
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(&models.Token{}, nil).Once()
		repo.On("InsertToken", &models.Token{}, u.ID).Return(nil).Once()
		repo.On("UpdateUser", u).Return(nil).Once()
		res, err := a.ResetPassword("token", u.Password)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Failed Reset - User not found", func(t *testing.T) {
		repo.On("FetchUserByToken", "invalid_token").Return(nil, errors.New("user not found")).Once()
		res, err := a.ResetPassword("invalid_token", "newPassword")
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Failed Reset - Error updating user", func(t *testing.T) {
		repo.On("FetchUserByToken", "token").Return(&u, nil).Once()
		mBcrypt.On("GenerateFromPassword", []byte(u.Password)).Return([]byte("verySecret"), nil).Once()
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(&models.Token{}, nil).Once()
		repo.On("InsertToken", &models.Token{}, u.ID).Return(nil).Once()
		repo.On("UpdateUser", u).Return(errors.New("update error")).Once()
		res, err := a.ResetPassword("token", u.Password)
		assert.Error(t, err)
		assert.Nil(t, res)
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
		repo.On("FetchUserById", u.ID).Return(&u, nil)
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(passwords.OldPassword)).Return(nil)
		mBcrypt.On("GenerateFromPassword", []byte(passwords.Password)).Return([]byte(passwords.Password), nil)
		repo.On("UpdateUser", models.User{ID: u.ID, Password: "newPassword"}).Return(nil)
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(&models.Token{}, nil)
		repo.On("InsertToken", &models.Token{}, u.ID).Return(nil)
		res, err := a.UpdatePassword(u.ID, passwords)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Failed Update - User not found", func(t *testing.T) {
		passwords := models.Passwords{
			Password:    "newPassword",
			OldPassword: "oldPassword",
		}
		u := models.User{
			ID: uuid.New(),
		}
		repo.On("FetchUserById", u.ID).Return(nil, errors.New("user not found"))
		res, err := a.UpdatePassword(u.ID, passwords)
		assert.Error(t, err)
		assert.Nil(t, res)
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
		repo.On("FetchUserById", u.ID).Return(&u, nil)
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(passwords.OldPassword)).Return(errors.New("wrong password"))
		res, err := a.UpdatePassword(u.ID, passwords)
		assert.Error(t, err)
		assert.Nil(t, res)
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
		repo.On("FetchUserById", u.ID).Return(&u, nil)
		mBcrypt.On("CompareHashAndPassword", []byte(u.Password), []byte(passwords.OldPassword)).Return(nil)
		mBcrypt.On("GenerateFromPassword", []byte(passwords.Password)).Return([]byte(passwords.Password), nil)
		repo.On("UpdateUser", models.User{ID: u.ID, Password: "newPassword"}).Return(errors.New("update error"))
		mToken.On("GenerateToken", u.ID, 24*time.Hour, token.ScopeAuthentication).Return(&models.Token{}, nil)
		repo.On("InsertToken", &models.Token{}, u.ID).Return(nil)
		res, err := a.UpdatePassword(u.ID, passwords)
		assert.Error(t, err)
		assert.Nil(t, res)
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
			PublicID: "publicId",
			URL:      "url",
		}
		repo.On("FetchAvatarById", u.ID).Return(avatar, nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, nil).Once()
		repo.On("DeleteAvatarById", avatar.PublicId).Return(nil).Once()
		cld.On("UploadToCloud", "avatar", "user.jpg").Return(&res, nil).Once()
		repo.On("InsertAvatar", mock.AnythingOfType("*models.Avatar")).Return(avatar, nil).Once()
		repo.On("UpdateUser", mock.Anything).Return(nil)
		err := a.UpdateProfile(u, "user.jpg")
		assert.NoError(t, err)
	})

	t.Run("Failed Update - User not found", func(t *testing.T) {
		repo.On("FetchAvatarById", u.ID).Return(models.Avatar{}, errors.New("user not found")).Once()
		err := a.UpdateProfile(u, "user.jpg")
		assert.Error(t, err)
	})

	t.Run("Failed Update - Error deleting old avatar", func(t *testing.T) {
		repo.On("FetchAvatarById", u.ID).Return(avatar, nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, errors.New("cloudinary error")).Once()
		err := a.UpdateProfile(u, "user.jpg")
		assert.Error(t, err)
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
		repo.On("FetchAvatarById", u.ID).Return(avatar, nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, nil).Once()
		repo.On("DeleteAvatarById", avatar.PublicId).Return(nil).Once()
		cld.On("UploadToCloud", "avatar", "user.jpg").Return(&res, errors.New("upload error")).Once()
		err := a.UpdateProfile(u, "user.jpg")
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
		repo.On("FetchAvatarById", u.ID).Return(avatar, nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, nil).Once()
		repo.On("DeleteAvatarById", avatar.PublicId).Return(nil).Once()
		cld.On("UploadToCloud", "avatar", "user.jpg").Return(&res, nil).Once()
		repo.On("InsertAvatar", &avatar).Return(avatar, errors.New("insert error")).Once()
		err := a.UpdateProfile(u, "user.jpg")
		assert.Error(t, err)
	})
}

// TestGetAllUsers tests the GetAllUsers use case for all success and error scenarios.
func TestGetAllUsers(t *testing.T) {
	a, _, repo, _, _, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		repo.On("FetchAllUsers").Return([]*models.User{}, nil).Once()
		users, err := a.GetAllUsers()
		assert.NoError(t, err)
		assert.NotNil(t, users)
	})

	t.Run("Failed to fetch users", func(t *testing.T) {
		repo.On("FetchAllUsers").Return(nil, errors.New("fetch error")).Once()
		users, err := a.GetAllUsers()
		assert.Error(t, err)
		assert.Nil(t, users)
	})
}

// TestGetUserDetails tests the GetUserDetails use case for all success and error scenarios.
func TestGetUserDetails(t *testing.T) {
	a, _, repo, _, _, _ := newTestAuthUC(t)

	t.Run("Success", func(t *testing.T) {
		id := uuid.New()
		repo.On("FetchUserById", id).Return(&models.User{}, nil)
		repo.On("FetchAvatarById", id).Return(models.Avatar{}, nil)
		user, err := a.GetUserDetails(id)
		assert.NoError(t, err)
		assert.NotNil(t, user)
	})

	t.Run("Failed - User not found", func(t *testing.T) {
		id := uuid.New()
		repo.On("FetchUserById", id).Return(nil, errors.New("user not found"))
		user, err := a.GetUserDetails(id)
		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("Failed - Error fetching avatar", func(t *testing.T) {
		id := uuid.New()
		repo.On("FetchUserById", id).Return(&models.User{}, nil)
		repo.On("FetchAvatarById", id).Return(models.Avatar{}, errors.New("avatar not found"))
		user, err := a.GetUserDetails(id)
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
		repo.On("FetchUserById", id).Return(&u, nil)
		repo.On("UpdateUser", u).Return(nil)
		res, err := a.UpdateUser(id, u)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Failed Update - User not found", func(t *testing.T) {
		id := uuid.New()
		u := models.User{
			ID:       id,
			Email:    "user@gmail.com",
			Password: "verySecret",
			Name:     "John Doe",
		}
		repo.On("FetchUserById", id).Return(nil, errors.New("user not found"))
		res, err := a.UpdateUser(id, u)
		assert.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("Failed Update - Error updating user", func(t *testing.T) {
		id := uuid.New()
		u := models.User{
			ID:       id,
			Email:    "user@gmail.com",
			Password: "verySecret",
			Name:     "John Doe",
		}
		repo.On("FetchUserById", id).Return(&u, nil)
		repo.On("UpdateUser", u).Return(errors.New("update error"))
		res, err := a.UpdateUser(id, u)
		assert.Error(t, err)
		assert.Nil(t, res)
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
		repo.On("FetchAvatarById", id).Return(avatar, nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, nil).Once()
		repo.On("DeleteAvatarById", avatar.PublicId).Return(nil).Once()
		repo.On("DeleteUserById", id).Return(nil).Once()
		err := a.DeleteUser(id)
		assert.NoError(t, err)
	})

	t.Run("Failed Delete - User not found", func(t *testing.T) {
		repo.On("FetchAvatarById", id).Return(models.Avatar{}, errors.New("user not found")).Once()
		err := a.DeleteUser(id)
		assert.Error(t, err)
	})

	t.Run("Failed Delete - Error deleting avatar", func(t *testing.T) {
		avatar := models.Avatar{
			PublicId: "publicId",
			Url:      "url",
			UserId:   id,
		}
		repo.On("FetchAvatarById", id).Return(avatar, nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, nil).Once()
		repo.On("DeleteAvatarById", avatar.PublicId).Return(errors.New("delete avatar error")).Once()
		err := a.DeleteUser(id)
		assert.Error(t, err)
	})

	t.Run("Failed Delete - Error deleting user", func(t *testing.T) {
		repo.On("FetchAvatarById", id).Return(avatar, nil).Once()
		cld.On("Destroy", avatar.PublicId).Return(&uploader.DestroyResult{}, nil).Once()
		repo.On("DeleteAvatarById", avatar.PublicId).Return(nil).Once()
		repo.On("DeleteUserById", id).Return(errors.New("delete error")).Once()
		err := a.DeleteUser(id)
		assert.Error(t, err)
	})
}

// TestLogout tests the DeleteUserToken use case for all success and error scenarios.
func TestLogout(t *testing.T) {
	a, _, repo, _, _, _ := newTestAuthUC(t)
	t.Run("Success", func(t *testing.T) {
		tok := "MQUYLLXB2PHU5PE6PG3HGG2AXI"
		id := uuid.New()
		repo.On("FetchUserByToken", tok).Return(&models.User{ID: id}, nil).Once()
		repo.On("DeleteTokenById", id).Return(nil).Once()
		err := a.DeleteUserToken(tok)
		assert.NoError(t, err)
	})

	t.Run("Failed Logout - Token not found", func(t *testing.T) {
		tok := "INVALIDTOKEN"
		repo.On("FetchUserByToken", tok).Return(nil, errors.New("token not found")).Once()
		err := a.DeleteUserToken(tok)
		assert.Error(t, err)
	})

	t.Run("Failed Logout - Error deleting token", func(t *testing.T) {
		tok := "MQUYLLXB2PHU5PE6PG3HGG2AXI"
		id := uuid.New()
		repo.On("FetchUserByToken", tok).Return(&models.User{ID: id}, nil).Once()
		repo.On("DeleteTokenById", id).Return(errors.New("delete error")).Once()
		err := a.DeleteUserToken(tok)
		assert.Error(t, err)
	})
}
