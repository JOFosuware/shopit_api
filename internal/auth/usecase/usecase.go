// Package usecase provides the business logic layer for authentication and user management.
//
// It implements methods for user, avatar, and token management, registration, login, password reset, profile updates, and user CRUD operations. The use case layer coordinates between repositories, token generation, password hashing, mail sending, and cloud storage to provide a complete authentication workflow for the application.
package usecase

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/auth"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/pkg/bcrypt"
	"github.com/jofosuware/go/shopit/pkg/cloudinary"
	"github.com/jofosuware/go/shopit/pkg/mailer"
	"github.com/jofosuware/go/shopit/pkg/token"
)

// AuthUC provides authentication and user management use cases.
// It should be constructed with all required dependencies.
type AuthUC struct {
	cld    cloudinary.CloudUploader // Cloud storage uploader
	repo   auth.Repo                // Data repository
	token  token.Tokener            // Token generator
	bcrypt bcrypt.Encryptor         // Password hasher
	mail   mailer.Mailer            // Mail sender
}

// NewAuthUC constructs a new AuthUC.
//
// Parameters:
//   - cld: a cloud storage uploader
//   - repo: a data repository
//   - token: a token generator
//   - b: a password hasher
//   - mail: a mail sender
//
// Returns:
//   - *AuthUC: a new AuthUC instance
func NewAuthUC(
	cld cloudinary.CloudUploader,
	repo auth.Repo,
	token token.Tokener,
	b bcrypt.Encryptor,
	mail mailer.Mailer,
) *AuthUC {
	return &AuthUC{
		cld:    cld,
		repo:   repo,
		token:  token,
		bcrypt: b,
		mail:   mail,
	}
}

// Register creates a new user, uploads avatar, and returns a user response with token.
// Returns the created user response or an error.
func (a *AuthUC) Register(user models.User, avatar string) (*models.UserResponse, error) {
	u, err := a.repo.FetchUserByEmail(user.Email)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return nil, fmt.Errorf("error fetching user: %v", err)
	}

	if err == nil && u.Email == user.Email {
		return nil, fmt.Errorf("user %s already exists", u.Name)
	}

	hashPassword, err := a.bcrypt.GenerateFromPassword([]byte(user.Password))
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %v", err)
	}

	user.Password = string(hashPassword)

	u, err = a.repo.InsertUser(user)
	if err != nil {
		return nil, fmt.Errorf("error saving user: %v", err)
	}

	res, err := a.cld.UploadToCloud("avatar", avatar)
	if err != nil {
		return nil, fmt.Errorf("error uploading to cloud: %v", err)
	}

	t, err := a.token.GenerateToken(u.ID, 24*time.Hour, token.ScopeAuthentication)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %v", err)
	}

	err = a.repo.InsertToken(t, u.ID)
	if err != nil {
		return nil, fmt.Errorf("error saving token: %v", err)
	}

	avtar := models.Avatar{
		PublicId: res.PublicID,
		Url:      res.URL,
		UserId:   u.ID,
	}

	avtar, err = a.repo.InsertAvatar(&avtar)
	if err != nil {
		return nil, fmt.Errorf("error saving avatar: %v", err)
	}

	u.Avatar = avtar

	ur := &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    *u,
	}

	return ur, nil
}

// Login authenticates a user and returns a user response with token.
// Returns the user response or an error.
func (a *AuthUC) Login(email, password string) (*models.UserResponse, error) {
	u, err := a.repo.FetchUserByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("error fetching user by email: %v", err)
	}

	if err := a.bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("error comparing password: %v", err)
	}

	t, err := a.token.GenerateToken(u.ID, 24*time.Hour, "authentication")
	if err != nil {
		return nil, fmt.Errorf("error generating token: %v", err)
	}

	if err = a.repo.InsertToken(t, u.ID); err != nil {
		return nil, fmt.Errorf("error saving token: %v", err)
	}

	avatar, err := a.repo.FetchAvatarById(u.ID)
	if err != nil {
		return nil, fmt.Errorf("error fetching avatar by id: %v", err)
	}

	u.Avatar = avatar

	ur := &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    *u,
	}

	return ur, nil
}

// SendPasswordResetEmail process password and email reset
func (a *AuthUC) SendPasswordResetEmail(email string, r *http.Request) (*models.Response, error) {
	var protocol string
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		protocol = forwarded
	} else if r.TLS != nil {
		protocol = "https"
	} else {
		protocol = "http"
	}
	if email == "" {
		return nil, errors.New("user must provide an email")
	}

	user, err := a.repo.FetchUserByEmail(email)
	if err != nil {
		return nil, err
	}

	// generate token
	t, err := a.token.GenerateToken(user.ID, 60*time.Minute, token.ScopeAuthentication)
	if err != nil {
		return nil, err
	}

	resetUrl := fmt.Sprintf("%s://%s/password/reset/%s", protocol, strings.Split(r.Host, ":")[0], t.PlainText)

	var data struct {
		Link string
	}

	data.Link = resetUrl

	//send mail
	err = a.mail.SendMail("DePeridot <postmaster@sandboxa7a6fd0db7744e4f8917325ae3ce1a04.mailgun.org>", email, "ShopIT Password Recovery", "password-reset", data)
	if err != nil {
		return nil, fmt.Errorf("error sending mail: %v", err)
	}

	// save token
	err = a.repo.InsertToken(t, user.ID)
	if err != nil {
		return nil, err
	}

	resp := models.Response{
		Success: true,
		Message: fmt.Sprintf("Email sent to %s", email),
	}

	return &resp, nil
}

// ResetPassword reset password
func (a *AuthUC) ResetPassword(newToken, password string) (*models.UserResponse, error) {
	// validate token
	if newToken == "" {
		return nil, errors.New("bad link")
	}

	// get user for token
	user, err := a.repo.FetchUserByToken(newToken)
	if err != nil {
		return nil, err
	}

	// hash password
	hashedPassword, err := a.bcrypt.GenerateFromPassword([]byte(password))
	if err != nil {
		return nil, err
	}

	// generate new token
	t, err := a.token.GenerateToken(user.ID, 24*time.Hour, token.ScopeAuthentication)
	if err != nil {
		return nil, err
	}

	// save new token
	err = a.repo.InsertToken(t, user.ID)
	if err != nil {
		return nil, err
	}

	// update password
	user.Password = string(hashedPassword)
	err = a.repo.UpdateUser(*user)
	if err != nil {
		return nil, err
	}

	resp := models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    *user,
	}

	return &resp, nil
}

// UpdatePassword updates the password of a user.
// Returns a user response or an error.
func (a *AuthUC) UpdatePassword(userId uuid.UUID, passwords models.Passwords) (*models.UserResponse, error) {
	var res *models.UserResponse

	// get user
	user, err := a.repo.FetchUserById(userId)
	if err != nil {
		return nil, err
	}

	// compare password
	err = a.bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(passwords.OldPassword))
	if err != nil {
		return nil, err
	}

	// hash password
	hashedPassword, err := a.bcrypt.GenerateFromPassword([]byte(passwords.Password))
	if err != nil {
		return nil, err
	}

	// generate new token
	t, err := a.token.GenerateToken(user.ID, 24*time.Hour, token.ScopeAuthentication)
	if err != nil {
		return nil, err
	}

	// save new token
	err = a.repo.InsertToken(t, user.ID)
	if err != nil {
		return nil, err
	}

	// update password
	user.Password = string(hashedPassword)
	err = a.repo.UpdateUser(*user)
	if err != nil {
		return nil, err
	}

	res = &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    *user,
	}

	return res, nil
}

// UpdateProfile updates the profile and avatar of a user.
// Returns an error if the update fails.
func (a *AuthUC) UpdateProfile(user models.User, avatar string) error {
	if avatar != "" {
		at, err := a.repo.FetchAvatarById(user.ID)
		if err != nil {
			return err
		}

		_, err = a.cld.Destroy(at.PublicId)
		if err != nil {
			return err
		}

		err = a.repo.DeleteAvatarById(at.PublicId)
		if err != nil {
			return err
		}

		res, err := a.cld.UploadToCloud("avatar", avatar)
		if err != nil {
			return err
		}

		at.PublicId = res.PublicID
		at.Url = res.URL
		at.UserId = user.ID

		_, err = a.repo.InsertAvatar(&at)
		if err != nil {
			return err
		}
	}

	err := a.repo.UpdateUser(user)
	if err != nil {
		return err
	}

	return nil
}

// GetAllUsers returns all users
func (a *AuthUC) GetAllUsers() ([]*models.User, error) {
	users, err := a.repo.FetchAllUsers()
	if err != nil {
		return nil, err
	}

	return users, nil
}

// GetUserDetails returns the details of a user by ID.
// Returns the user or an error.
func (a *AuthUC) GetUserDetails(userID uuid.UUID) (*models.User, error) {
	user, err := a.repo.FetchUserById(userID)
	if err != nil {
		return nil, err
	}

	avatar, err := a.repo.FetchAvatarById(userID)
	if err != nil {
		return nil, err
	}
	user.Avatar = avatar

	return user, nil
}

// UpdateUser updates the details of a user by ID.
// Returns a user response or an error.
func (a *AuthUC) UpdateUser(userID uuid.UUID, user models.User) (*models.UserResponse, error) {
	// get user
	u, err := a.repo.FetchUserById(userID)
	if err != nil {
		return nil, err
	}
	u.Name = user.Name
	u.Email = user.Email
	u.Role = user.Role

	err = a.repo.UpdateUser(*u)
	if err != nil {
		return nil, err
	}

	res := &models.UserResponse{
		Success: true,
	}

	return res, nil
}

// DeleteUser deletes a user
func (a *AuthUC) DeleteUser(userID uuid.UUID) error {
	avatar, err := a.repo.FetchAvatarById(userID)
	if err != nil {
		return err
	}

	_, err = a.cld.Destroy(avatar.PublicId)
	if err != nil {
		return err
	}

	err = a.repo.DeleteAvatarById(avatar.PublicId)
	if err != nil {
		return err
	}

	err = a.repo.DeleteUserById(userID)
	if err != nil {
		return err
	}

	return nil
}

// DeleteUserToken deletes user token from the database
func (a *AuthUC) DeleteUserToken(token string) error {
	user, err := a.repo.FetchUserByToken(token)
	if err != nil {
		return err
	}

	err = a.repo.DeleteTokenById(user.ID)
	if err != nil {
		return err
	}

	return nil
}
