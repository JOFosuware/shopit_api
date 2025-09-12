package models

import (
	"time"

	"github.com/google/uuid"
)

// Token is the type for authentication tokens
type Token struct {
	ID        uuid.UUID `json:"-"`
	PlainText string    `json:"token"`
	UserID    uuid.UUID `json:"-"`
	Hash      []byte    `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}
