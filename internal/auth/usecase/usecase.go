// internal/auth/usecase/usecase.go
// Package usecase provides authentication and user-management business logic.
//
// It coordinates repositories, token generation, password hashing, mail sending, and
// cloud storage to implement registration, login, password reset, profile updates,
// and user CRUD operations.
package usecase

import (
	"context"
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

// mapUserToPublic converts the internal, full User model to the public response model.
// This ensures password data is never serialized in API responses.
func mapUserToPublic(u *models.User) models.UserPublic {
	return models.UserPublic{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Role:      u.Role,
		Avatar:    u.Avatar,
		CreatedAt: u.CreatedAt,
	}
}

// Register creates a new user, uploads avatar, and returns a user response with token.
func (a *AuthUC) Register(user models.User, avatar string) (*models.UserResponse, error) {
	// 1. Check for existing user
	u, err := a.repo.FetchUserByEmail(user.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error fetching user: %w", err)
	}

	if err == nil {
		return nil, fmt.Errorf("%w: %s", auth.ErrUserAlreadyExists, user.Email)
	}
	
	// 2. Hash Password
	hashPassword, err := a.bcrypt.GenerateFromPassword([]byte(user.Password))
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}
	user.Password = string(hashPassword)

	// 3. Insert User (Initial DB persistence)
	u, err = a.repo.InsertUser(user)
	if err != nil {
		return nil, fmt.Errorf("error saving user: %w", err)
	}

	// 4. Upload Avatar (External Dependency - Non-atomic)
	res, err := a.cld.UploadToCloud("avatar", avatar)
	if err != nil {
		// CRITICAL ROLLBACK: If cloud fails, delete the user we just inserted.
		a.repo.DeleteUserById(u.ID)
		return nil, errors.Join(fmt.Errorf("error uploading to cloud: %w", err), a.repo.DeleteUserById(u.ID))
	}
	
	// 5. Generate Token
	t, err := a.token.GenerateToken(u.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
	if err != nil {
		// CRITICAL ROLLBACK: If token fails, delete the user and clean up the cloud resource.
		a.destroyAvatar(res.PublicID)
		a.repo.DeleteUserById(u.ID)
		return nil, errors.Join(fmt.Errorf("failed to generate token: %w", err), a.destroyAvatar(res.PublicID), a.repo.DeleteUserById(u.ID))
	}
	
	// 6. Atomically persist Avatar and Token using the new repository method.
	// We use the placeholder avatar info, knowing we will finalize it immediately after.
	avatarModel := models.Avatar{
		PublicId: res.PublicID,
		Url:      res.URL,
		UserId:   u.ID,
	}
	tokenModel := &models.Token{
		Hash: t.Hash,
		Expiry: t.Expiry,
		Scope: pkgtoken.ScopeAuthentication,
	}

	// Atomic DB step: Insert Avatar (placeholder) and Token together
	_, err = a.repo.CreateUserAtomically(context.Background(), user, avatarModel, tokenModel) // NOTE: This assumes atomic repo now handles Token/Avatar insertion based on UserID
	if err != nil {
		// CRITICAL ROLLBACK: If DB insertion fails (even if token/avatar inserts fail), delete the user.
		a.destroyAvatar(res.PublicID)
		a.repo.DeleteUserById(u.ID)
		return nil, fmt.Errorf("atomic DB write failed: %w", err)
	}
	
	// 7. Return Response
	u.Avatar = avatarModel
	ur := &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    mapUserToPublic(u),
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
		User:    mapUserToPublic(u),
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

	resp := &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    mapUserToPublic(user), // Mapped here
	}

	return resp, nil
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
		User:    mapUserToPublic(user), // Mapped here
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

		// Cleanup old avatar
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

	// Only update non-password fields in the repo
	err := a.repo.UpdateUser(user)
	if err != nil {
		return err
	}

	return nil
}

// GetAllUsers returns all users.
// This now returns UserPublic models to prevent password leakage.
func (a *AuthUC) GetAllUsers(actor *models.User) ([]*models.UserPublic, error) {
	if actor == nil || actor.Role != "admin" {
		return nil, auth.ErrForbidden
	}

	users, err := a.repo.FetchAllUsers()
	if err != nil {
		return nil, err
	}

	// Map all results to public models
	publicUsers := make([]*models.UserPublic, len(users))
	for i, u := range users {
		// Explicitly create a new struct instance, and then assign the pointer to it.
		publicUsers[i] = &models.UserPublic{
			ID:        u.ID,
			Name:      u.Name,
			Email:     u.Email,
			Role:      u.Role,
			Avatar:    u.Avatar,
			CreatedAt: u.CreatedAt,
		}
	}
	return publicUsers, nil
}

// GetUserDetails returns the details of a user by ID.
// This now returns UserPublic model.
func (a *AuthUC) GetUserDetails(actor *models.User, userID uuid.UUID) (*models.UserPublic, error) {
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

	// Return public model
	userPublic := mapUserToPublic(user)
	userPublic.Avatar = avatar
	return &userPublic, nil
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
		User:    mapUserToPublic(u),
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