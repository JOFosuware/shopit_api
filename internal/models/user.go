package models

import (
	"time"

	"github.com/google/uuid"
)

// User full model
type User struct {
	ID        uuid.UUID
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
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
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	User    User   `json:"user,omitempty"`
}

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type Passwords struct {
	Password    string
	OldPassword string
}
