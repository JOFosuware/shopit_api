package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents the internal, full model used for database persistence.
type User struct {
	ID        uuid.UUID
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Sensitive field, hidden from JSON by default
	Role      string    `json:"role"`
	Avatar    Avatar    `json:"avatar"`
	CreatedAt time.Time `json:"createdAt"`
}

// UserPublic is the model used for API responses, containing only non-sensitive user data.
type UserPublic struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Avatar    Avatar    `json:"avatar"`
	CreatedAt time.Time `json:"createdAt"`
}

// Avatar model
type Avatar struct {
	PublicId string `json:"publicId"`
	Url      string `json:"url"`
	UserId   uuid.UUID
}

type UserResponse struct {
	Success bool       `json:"success"`
	Token   string     `json:"token,omitempty"`
	User    UserPublic `json:"user,omitempty"` // CHANGE: Use UserPublic here
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type Passwords struct {
	Password    string
	OldPassword string
}
