package auth

import (
	"errors"
)

// ErrUserNotFound is returned when a user is not found.
var ErrUserNotFound = errors.New("user not found")

// ErrInvalidPassword is returned when the provided password is invalid.
var ErrInvalidPassword = errors.New("invalid password")

// ErrUserAlreadyExists is returned when a user with the same email already exists.
var ErrUserAlreadyExists = errors.New("user already exists")

// ErrUserUpdateFailed is returned when there is an error updating a user.
var ErrUserUpdateFailed = errors.New("failed to update user")

// ErrUserDeletionFailed is returned when there is an error deleting a user.
var ErrUserDeletionFailed = errors.New("failed to delete user")

// ErrUserAccessDenied is returned when a user does not have access to a resource.
var ErrUserAccessDenied = errors.New("access denied")

// ErrTokenNotFound is returned when a token is not found.
var ErrTokenNotFound = errors.New("token not found")

// ErrAvatarUploadFailed is returned when there is an error uploading an avatar.
var ErrAvatarUploadFailed = errors.New("failed to upload avatar")

// ErrAvatarDeletionFailed is returned when there is an error deleting an avatar.
var ErrAvatarDeletionFailed = errors.New("failed to delete old avatar")

// ErrInvalidOldPassword is returned when the provided old password is invalid.
var ErrInvalidOldPassword = errors.New("invalid old password")

// ErrPasswordUpdateFailed is returned when there is an error updating a password.
var ErrPasswordUpdateFailed = errors.New("failed to update password")

// ErrForbidden is returned when a user is not authorized to perform an action.
var ErrForbidden = errors.New("forbidden")
