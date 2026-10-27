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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/auth"
	"github.com/jofosuware/go/shopit/internal/models"
	"github.com/jofosuware/go/shopit/pkg/bcrypt"
	"github.com/jofosuware/go/shopit/pkg/cloudinary"
	"github.com/jofosuware/go/shopit/pkg/mailer"
	pkgtoken "github.com/jofosuware/go/shopit/pkg/token"
	cryptobcrypt "golang.org/x/crypto/bcrypt"
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

// Register creates a user with its avatar and authentication token.
func (a *AuthUC) Register(ctx context.Context, user models.User, avatar string) (*models.UserResponse, error) {
	_, err := a.repo.FetchUserByEmail(ctx, user.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error fetching user: %w", err)
	}
	if err == nil {
		return nil, auth.ErrUserAlreadyExists
	}

	hashedPassword, err := a.bcrypt.GenerateFromPassword([]byte(user.Password))
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}
	user.Password = string(hashedPassword)
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	upload, err := a.cld.UploadToCloud("avatar", avatar)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", auth.ErrAvatarUploadFailed, err)
	}
	if upload == nil {
		return nil, fmt.Errorf("%w: cloud uploader returned no avatar", auth.ErrAvatarUploadFailed)
	}

	t, err := a.token.GenerateToken(user.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to generate token: %w", err), a.destroyAvatar(upload.PublicID))
	}
	if t == nil {
		return nil, errors.Join(errors.New("token generator returned no token"), a.destroyAvatar(upload.PublicID))
	}

	avatarModel := models.Avatar{
		PublicId: upload.PublicID,
		Url:      upload.URL,
	}
	t.Scope = pkgtoken.ScopeAuthentication
	createdUser, err := a.repo.CreateUserAtomically(ctx, user, avatarModel, t)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("error creating user: %w", err), a.destroyAvatar(upload.PublicID))
	}
	if createdUser == nil {
		return nil, errors.Join(errors.New("repository returned no created user"), a.destroyAvatar(upload.PublicID))
	}

	avatarModel.UserId = createdUser.ID
	t.UserID = createdUser.ID
	response := &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    mapUserToPublic(createdUser),
	}
	response.User.Avatar = avatarModel
	return response, nil
}

func (a *AuthUC) destroyAvatar(publicID string) error {
	if publicID == "" {
		return nil
	}
	_, err := a.cld.Destroy(publicID)
	if err != nil {
		return fmt.Errorf("destroying avatar %q: %w", publicID, err)
	}
	return nil
}

// Authenticate resolves an unexpired authentication token to its user.
func (a *AuthUC) Authenticate(ctx context.Context, plainTextToken string) (*models.User, error) {
	if plainTextToken == "" {
		return nil, auth.ErrTokenNotFound
	}

	user, err := a.repo.FetchUserByTokenWithScope(ctx, plainTextToken, pkgtoken.ScopeAuthentication)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %w", auth.ErrTokenNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("fetching authenticated user: %w", err)
	}
	if user == nil {
		return nil, errors.New("repository returned no user for authentication token")
	}
	return user, nil
}

// Login authenticates a user and returns a user response with token.
func (a *AuthUC) Login(ctx context.Context, email, password string) (*models.UserResponse, error) {
	u, err := a.repo.FetchUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", auth.ErrUserNotFound, err)
		}
		return nil, fmt.Errorf("error fetching user by email: %w", err)
	}
	if u == nil {
		return nil, errors.New("repository returned no user")
	}

	if err := a.bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		if errors.Is(err, cryptobcrypt.ErrMismatchedHashAndPassword) {
			return nil, fmt.Errorf("%w: %w", auth.ErrInvalidPassword, err)
		}
		return nil, fmt.Errorf("verifying password: %w", err)
	}

	avatar, err := a.repo.FetchAvatarById(ctx, u.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error fetching avatar by id: %w", err)
	}

	u.Avatar = avatar

	t, err := a.token.GenerateToken(u.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
	if err != nil {
		return nil, fmt.Errorf("error generating token: %w", err)
	}
	if t == nil {
		return nil, errors.New("token generator returned no token")
	}
	t.Scope = pkgtoken.ScopeAuthentication
	if err = a.repo.InsertToken(ctx, t, u.ID); err != nil {
		return nil, fmt.Errorf("error saving token: %w", err)
	}

	return &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    mapUserToPublic(u),
	}, nil
}

// SendPasswordResetEmail sends a password reset email to the given address.
func (a *AuthUC) SendPasswordResetEmail(ctx context.Context, email string) (*models.Response, error) {
	if email == "" {
		return nil, errors.New("user must provide an email")
	}

	if a.frontendURL == "" || a.emailFrom == "" {
		return nil, errors.New("password reset is not configured")
	}

	user, err := a.repo.FetchUserByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return passwordResetAcceptedResponse(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("error fetching user by email: %w", err)
	}
	if user == nil {
		return nil, errors.New("repository returned no user")
	}

	t, err := a.token.GenerateToken(user.ID, 60*time.Minute, pkgtoken.ScopePasswordReset)
	if err != nil {
		return nil, fmt.Errorf("error generating token: %w", err)
	}
	if t == nil {
		return nil, errors.New("token generator returned no token")
	}
	t.Scope = pkgtoken.ScopePasswordReset

	resetURL := fmt.Sprintf("%s/password/reset/%s", a.frontendURL, t.PlainText)
	data := struct {
		Link string
	}{Link: resetURL}

	if err = a.repo.InsertToken(ctx, t, user.ID); err != nil {
		return nil, fmt.Errorf("error saving token: %w", err)
	}
	if err = a.mail.SendMail(a.emailFrom, email, "ShopIT Password Recovery", "password-reset", data); err != nil {
		return nil, fmt.Errorf("error sending mail: %w", err)
	}

	return passwordResetAcceptedResponse(), nil
}

func passwordResetAcceptedResponse() *models.Response {
	return &models.Response{
		Success: true,
		Message: "If an account exists for that email, password reset instructions will be sent.",
	}
}

// ResetPassword resets a user's password using the provided token.
func (a *AuthUC) ResetPassword(ctx context.Context, newToken, password string) (*models.UserResponse, error) {
	if newToken == "" || password == "" {
		return nil, errors.New("bad link")
	}

	user, err := a.repo.FetchUserByTokenWithScope(ctx, newToken, pkgtoken.ScopePasswordReset)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", auth.ErrTokenNotFound, err)
		}
		return nil, fmt.Errorf("fetching user for password reset: %w", err)
	}
	if user == nil {
		return nil, errors.New("repository returned no user for password reset token")
	}

	hashedPassword, err := a.bcrypt.GenerateFromPassword([]byte(password))
	if err != nil {
		return nil, fmt.Errorf("hashing password for reset: %w", err)
	}

	t, err := a.token.GenerateToken(user.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
	if err != nil {
		return nil, fmt.Errorf("generating authentication token after password reset: %w", err)
	}
	if t == nil {
		return nil, errors.New("token generator returned no token")
	}
	t.Scope = pkgtoken.ScopeAuthentication

	user.Password = string(hashedPassword)
	if err = a.repo.UpdatePasswordAndReplaceToken(ctx, *user, t, true); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", auth.ErrTokenNotFound, err)
		}
		return nil, fmt.Errorf("%w: %w", auth.ErrPasswordUpdateFailed, err)
	}

	return &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    mapUserToPublic(user),
	}, nil
}

// UpdatePassword updates the user's password and rotates their authentication token.
func (a *AuthUC) UpdatePassword(ctx context.Context, userID uuid.UUID, passwords models.Passwords) (*models.UserResponse, error) {
	user, err := a.repo.FetchUserById(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", auth.ErrUserNotFound, err)
		}
		return nil, fmt.Errorf("fetching user for password update: %w", err)
	}
	if user == nil {
		return nil, errors.New("repository returned no user")
	}

	if err = a.bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(passwords.OldPassword)); err != nil {
		if errors.Is(err, cryptobcrypt.ErrMismatchedHashAndPassword) {
			return nil, fmt.Errorf("%w: %w", auth.ErrInvalidOldPassword, err)
		}
		return nil, fmt.Errorf("verifying current password: %w", err)
	}

	hashedPassword, err := a.bcrypt.GenerateFromPassword([]byte(passwords.Password))
	if err != nil {
		return nil, fmt.Errorf("hashing new password: %w", err)
	}

	t, err := a.token.GenerateToken(user.ID, 24*time.Hour, pkgtoken.ScopeAuthentication)
	if err != nil {
		return nil, fmt.Errorf("generating authentication token after password update: %w", err)
	}
	if t == nil {
		return nil, errors.New("token generator returned no token")
	}
	t.Scope = pkgtoken.ScopeAuthentication

	user.Password = string(hashedPassword)
	if err = a.repo.UpdatePasswordAndReplaceToken(ctx, *user, t, false); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", auth.ErrUserNotFound, err)
		}
		return nil, fmt.Errorf("%w: %w", auth.ErrPasswordUpdateFailed, err)
	}

	return &models.UserResponse{
		Success: true,
		Token:   t.PlainText,
		User:    mapUserToPublic(user),
	}, nil
}

// UpdateProfile updates the profile and, when provided, replaces the avatar.
func (a *AuthUC) UpdateProfile(ctx context.Context, user models.User, avatar string) error {
	if avatar == "" {
		if err := a.repo.UpdateProfile(ctx, user.ID, user.Name, user.Email, nil); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: %w", auth.ErrUserNotFound, err)
			}
			return fmt.Errorf("updating profile: %w", err)
		}
		return nil
	}

	oldAvatar, err := a.repo.FetchAvatarById(ctx, user.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("fetching current avatar: %w", err)
	}
	hasOldAvatar := err == nil

	uploaded, err := a.cld.UploadToCloud("avatar", avatar)
	if err != nil {
		return fmt.Errorf("%w: %w", auth.ErrAvatarUploadFailed, err)
	}
	if uploaded == nil {
		return fmt.Errorf("%w: cloud uploader returned no avatar", auth.ErrAvatarUploadFailed)
	}

	newAvatar := models.Avatar{
		PublicId: uploaded.PublicID,
		Url:      uploaded.URL,
		UserId:   user.ID,
	}
	if err = a.repo.UpdateProfile(ctx, user.ID, user.Name, user.Email, &newAvatar); err != nil {
		cleanupErr := a.destroyAvatar(newAvatar.PublicId)
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("cleaning up uploaded avatar: %w", cleanupErr)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return errors.Join(fmt.Errorf("%w: %w", auth.ErrUserNotFound, err), cleanupErr)
		}
		return errors.Join(fmt.Errorf("persisting profile and new avatar: %w", err), cleanupErr)
	}

	if hasOldAvatar && oldAvatar.PublicId != newAvatar.PublicId {
		if err = a.destroyAvatar(oldAvatar.PublicId); err != nil {
			return fmt.Errorf("%w: new avatar is active, but old avatar cleanup failed: %w", auth.ErrAvatarDeletionFailed, err)
		}
	}

	return nil
}

// GetAllUsers returns public user data for an administrator.
func (a *AuthUC) GetAllUsers(ctx context.Context, actor *models.User) ([]*models.UserPublic, error) {
	if actor == nil || actor.Role != "admin" {
		return nil, auth.ErrForbidden
	}

	users, err := a.repo.FetchAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching all users: %w", err)
	}
	return users, nil
}

// GetUserDetails returns public details for the requested user.
func (a *AuthUC) GetUserDetails(ctx context.Context, actor *models.User, userID uuid.UUID) (*models.UserPublic, error) {
	if actor == nil || (actor.Role != "admin" && actor.ID != userID) {
		return nil, auth.ErrUserAccessDenied
	}

	user, err := a.repo.FetchUserById(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", auth.ErrUserNotFound, err)
		}
		return nil, fmt.Errorf("fetching user details: %w", err)
	}
	if user == nil {
		return nil, errors.New("repository returned no user")
	}

	avatar, err := a.repo.FetchAvatarById(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("fetching user avatar: %w", err)
	}
	user.Avatar = avatar

	publicUser := mapUserToPublic(user)
	return &publicUser, nil
}

// UpdateUser updates a user's public profile fields.
func (a *AuthUC) UpdateUser(ctx context.Context, actor *models.User, userID uuid.UUID, user models.User) (*models.UserResponse, error) {
	if actor == nil || actor.Role != "admin" {
		return nil, auth.ErrForbidden
	}

	existing, err := a.repo.FetchUserById(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", auth.ErrUserNotFound, err)
		}
		return nil, fmt.Errorf("fetching user for update: %w", err)
	}
	if existing == nil {
		return nil, errors.New("repository returned no user")
	}
	existing.Name = user.Name
	existing.Email = user.Email
	existing.Role = user.Role

	if err = a.repo.UpdateUser(ctx, *existing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %w", auth.ErrUserNotFound, err)
		}
		return nil, fmt.Errorf("%w: %w", auth.ErrUserUpdateFailed, err)
	}

	return &models.UserResponse{
		Success: true,
		User:    mapUserToPublic(existing),
	}, nil
}

// DeleteUser deletes a user and their avatar.
func (a *AuthUC) DeleteUser(ctx context.Context, actor *models.User, userID uuid.UUID) error {
	if actor == nil || actor.Role != "admin" {
		return auth.ErrForbidden
	}

	avatar, avatarErr := a.repo.FetchAvatarById(ctx, userID)
	if avatarErr != nil && !errors.Is(avatarErr, sql.ErrNoRows) {
		return fmt.Errorf("fetching avatar for user deletion: %w", avatarErr)
	}

	if err := a.repo.DeleteUserById(ctx, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %w", auth.ErrUserNotFound, err)
		}
		return fmt.Errorf("%w: %w", auth.ErrUserDeletionFailed, err)
	}

	if avatarErr == nil && avatar.PublicId != "" {
		if _, err := a.cld.Destroy(avatar.PublicId); err != nil {
			return fmt.Errorf("%w: user was deleted but avatar cleanup failed: %w", auth.ErrAvatarDeletionFailed, err)
		}
	}

	return nil
}

// DeleteUserToken deletes only the presented authentication token.
func (a *AuthUC) DeleteUserToken(ctx context.Context, plainTextToken string) error {
	err := a.repo.DeleteAuthenticationToken(ctx, plainTextToken)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %w", auth.ErrTokenNotFound, err)
	}
	if err != nil {
		return fmt.Errorf("deleting authentication token: %w", err)
	}
	return nil
}
