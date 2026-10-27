package auth

import "errors"

var (
	// ErrUserNotFound indicates that the requested user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidPassword indicates that the supplied login password does not match.
	ErrInvalidPassword = errors.New("invalid password")
	// ErrUserAlreadyExists indicates an attempt to use an email address already in use.
	ErrUserAlreadyExists = errors.New("user already exists")
	// ErrUserUpdateFailed indicates that persisting a user update failed.
	ErrUserUpdateFailed = errors.New("failed to update user")
	// ErrUserDeletionFailed indicates that deleting a user failed.
	ErrUserDeletionFailed = errors.New("failed to delete user")
	// ErrUserAccessDenied indicates that the caller cannot access the requested user.
	ErrUserAccessDenied = errors.New("access denied")
	// ErrTokenNotFound indicates that a token is missing, invalid, or expired.
	ErrTokenNotFound = errors.New("token not found")
	// ErrAvatarUploadFailed indicates that uploading an avatar failed.
	ErrAvatarUploadFailed = errors.New("failed to upload avatar")
	// ErrAvatarDeletionFailed indicates that cleanup of an active user's former avatar failed.
	ErrAvatarDeletionFailed = errors.New("failed to delete old avatar")
	// ErrInvalidOldPassword indicates that the supplied current password does not match.
	ErrInvalidOldPassword = errors.New("invalid old password")
	// ErrPasswordUpdateFailed indicates that persisting a password update failed.
	ErrPasswordUpdateFailed = errors.New("failed to update password")
	// ErrForbidden indicates that the caller is not allowed to perform the operation.
	ErrForbidden = errors.New("forbidden")
)
