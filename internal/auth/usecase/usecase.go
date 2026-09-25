// Package usecase provides authentication and user-management business logic.
//
// It coordinates repositories, token generation, password hashing, mail sending, and
// cloud storage to implement registration, login, password reset, profile updates,
// and user CRUD operations.
package usecase

import (
	"database/sql"
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
	pkgtoken "github.com/jofosuware/go/shopit/pkg/token"
)

// AuthUC provides authentication and user management use cases.
// It should be constructed with all required dependencies.
type AuthUC struct {
	cld         cloudinary.CloudUploader
	repo        auth.Repo
	token       pkgtoken.Tokener
	bcrypt      bcrypt.Encryptor
	mail        mailer.Mailer
	frontendURL string
	emailFrom   string
}

// NewAuthUC returns a new AuthUC with the provided dependencies.
func NewAuthUC(
	cld cloudinary.CloudUploader,
	repo auth.Repo,
	token pkgtoken.Tokener,
	b bcrypt.Encryptor,
	mail mailer.Mailer,
	frontendURL string,
	emailFrom string,
) *AuthUC {
	return &AuthUC{
		cld:         cld,
		repo:        repo,
		token:       token,
		bcrypt:      b,
		mail:        mail,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		emailFrom:   emailFrom,
	}
}

// Register creates a new user, uploads avatar, and returns a user response with token.
func (a *AuthUC) Register(user models.User, avatar string) (*models.UserResponse, error) {
	u, err := a.repo.FetchUserByEmail(user.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error fetching user: %w", err)
	}

	if err == nil {
		return nil, fmt.Errorf("%w: %s", auth.ErrUserAlreadyExists, user.Email)
	}

	hashPassword, err := a.bcrypt.GenerateFromPassword([]byte(user.Password))
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}

	user.Password = string(hashPassword)

	u, err = a.repo.InsertUser(user)
	if err != nil {
		return nil, fmt.Errorf("error saving user: %w", err)
	}

	res, err := a.cld.UploadToCloud("avatar", avatar)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("error uploading to cloud: %w", err), a.repo.DeleteUserById(u.ID))
	}

	t, err := a.token.GenerateToken(u.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to generate token: %w", err), a.destroyAvatar(res.PublicID), a.repo.DeleteUserById(u.ID))
	}

	err = a.repo.InsertToken(t, u.ID)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("error saving token: %w", err), a.destroyAvatar(res.PublicID), a.repo.DeleteUserById(u.ID))
	}

	avtar := models.Avatar{
		PublicId: res.PublicID,
		Url:      res.URL,
		UserId:   u.ID,
	}

	avtar, err = a.repo.InsertAvatar(&avtar)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("error saving avatar: %w", err), a.destroyAvatar(res.PublicID), a.repo.DeleteUserById(u.ID))
	}

	u.Avatar = avtar

	ur := &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    *u,
	}

	return ur, nil
}

func (a *AuthUC) destroyAvatar(publicID string) error {
	_, err := a.cld.Destroy(publicID)
	return err
}

// Login authenticates a user and returns a user response with token.
func (a *AuthUC) Login(email, password string) (*models.UserResponse, error) {
	u, err := a.repo.FetchUserByEmail(email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %v", auth.ErrUserNotFound, err)
		}
		return nil, fmt.Errorf("error fetching user by email: %w", err)
	}

	if err := a.bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("%w: %v", auth.ErrInvalidPassword, err)
	}

	t, err := a.token.GenerateToken(u.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
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

// SendPasswordResetEmail sends a password reset email to the given address.
func (a *AuthUC) SendPasswordResetEmail(email string, r *http.Request) (*models.Response, error) {
	if email == "" {
		return nil, errors.New("user must provide an email")
	}

	user, err := a.repo.FetchUserByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("error fetching user by email: %w", err)
	}

	// generate password-reset token (distinct scope)
	t, err := a.token.GenerateToken(user.ID, 60*time.Minute, pkgtoken.ScopePasswordReset)
	if err != nil {
		return nil, fmt.Errorf("error generating token: %w", err)
	}

	if a.frontendURL == "" || a.emailFrom == "" {
		return nil, errors.New("password reset is not configured")
	}
	resetURL := fmt.Sprintf("%s/password/reset/%s", a.frontendURL, t.PlainText)

	var data struct {
		Link string
	}

	data.Link = resetURL

	// Persist the token before sending its link so recipients never receive an unusable token.
	err = a.repo.InsertToken(t, user.ID)
	if err != nil {
		return nil, fmt.Errorf("error saving token: %w", err)
	}

	err = a.mail.SendMail(a.emailFrom, email, "ShopIT Password Recovery", "password-reset", data)
	if err != nil {
		return nil, fmt.Errorf("error sending mail: %w", err)
	}

	resp := models.Response{
		Success: true,
		Message: fmt.Sprintf("Email sent to %s", email),
	}

	return &resp, nil
}

// ResetPassword resets a user's password using the provided token.
func (a *AuthUC) ResetPassword(newToken, password string) (*models.UserResponse, error) {
	// validate token
	if newToken == "" || password == "" {
		return nil, errors.New("bad link")
	}

	// get user for token - require repository method that enforces scope
	user, err := a.repo.FetchUserByTokenWithScope(newToken, pkgtoken.ScopePasswordReset)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %v", auth.ErrTokenNotFound, err)
		}
		return nil, err
	}

	// hash password
	hashedPassword, err := a.bcrypt.GenerateFromPassword([]byte(password))
	if err != nil {
		return nil, err
	}

	// generate a fresh authentication token after successful password reset
	t, err := a.token.GenerateToken(user.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
	if err != nil {
		return nil, err
	}

	user.Password = string(hashedPassword)
	if err = a.repo.InsertToken(t, user.ID); err != nil {
		return nil, err
	}

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
func (a *AuthUC) UpdatePassword(userId uuid.UUID, passwords models.Passwords) (*models.UserResponse, error) {
	// get user
	user, err := a.repo.FetchUserById(userId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %v", auth.ErrUserNotFound, err)
		}
		return nil, err
	}

	// compare password
	err = a.bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(passwords.OldPassword))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", auth.ErrInvalidOldPassword, err)
	}

	// hash password
	hashedPassword, err := a.bcrypt.GenerateFromPassword([]byte(passwords.Password))
	if err != nil {
		return nil, err
	}

	// generate new token
	t, err := a.token.GenerateToken(user.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
	if err != nil {
		return nil, err
	}

	user.Password = string(hashedPassword)
	if err = a.repo.InsertToken(t, user.ID); err != nil {
		return nil, err
	}

	err = a.repo.UpdateUser(*user)
	if err != nil {
		return nil, err
	}

	res := &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    *user,
	}

	return res, nil
}

// UpdateProfile updates the profile and avatar of a user.
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

// GetAllUsers returns all users.
func (a *AuthUC) GetAllUsers(actor *models.User) ([]*models.User, error) {
	if actor == nil || actor.Role != "admin" {
		return nil, auth.ErrForbidden
	}

	users, err := a.repo.FetchAllUsers()
	if err != nil {
		return nil, err
	}

	return users, nil
}

// GetUserDetails returns the details of a user by ID.
func (a *AuthUC) GetUserDetails(actor *models.User, userID uuid.UUID) (*models.User, error) {
	// allow users to fetch their own details or admin to fetch any
	if actor == nil {
		return nil, auth.ErrUserAccessDenied
	}
	if actor.Role != "admin" && actor.ID != userID {
		return nil, auth.ErrUserAccessDenied
	}

	user, err := a.repo.FetchUserById(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %v", auth.ErrUserNotFound, err)
		}
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
func (a *AuthUC) UpdateUser(actor *models.User, userID uuid.UUID, user models.User) (*models.UserResponse, error) {
	if actor == nil || actor.Role != "admin" {
		return nil, auth.ErrForbidden
	}

	// get user
	u, err := a.repo.FetchUserById(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %v", auth.ErrUserNotFound, err)
		}
		return nil, err
	}
	u.Name = user.Name
	u.Email = user.Email
	u.Role = user.Role

	err = a.repo.UpdateUser(*u)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", auth.ErrUserUpdateFailed, err)
	}

	res := &models.UserResponse{
		Success: true,
	}

	return res, nil
}

// DeleteUser deletes a user
func (a *AuthUC) DeleteUser(actor *models.User, userID uuid.UUID) error {
	if actor == nil || actor.Role != "admin" {
		return auth.ErrForbidden
	}

	avatar, err := a.repo.FetchAvatarById(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %v", auth.ErrUserNotFound, err)
		}
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
		return fmt.Errorf("%w: %v", auth.ErrUserDeletionFailed, err)
	}

	return nil
}

// DeleteUserToken deletes user token from the database
func (a *AuthUC) DeleteUserToken(token string) error {
	// token used to logout must be an authentication token
	user, err := a.repo.FetchUserByTokenWithScope(token, pkgtoken.ScopeAuthentication)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %v", auth.ErrTokenNotFound, err)
		}
		return err
	}

	err = a.repo.DeleteTokenById(user.ID)
	if err != nil {
		return err
	}

	return nil
}
