package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jofosuware/go/shopit/internal/models"
)

const (
	ScopeAuthentication = "authentication"
)

type Tokener interface {
	GenerateToken(userID uuid.UUID, ttl time.Duration, scope string) (*models.Token, error)
}

// Token is the type for authentication Tokens
type Token struct {
}

func NewToken() *Token {
	return &Token{}
}

// GenerateToken generates a Token that lasts for ttl, and returns it
func (t *Token) GenerateToken(userID uuid.UUID, ttl time.Duration, scope string) (*models.Token, error) {
	token := &models.Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	token.PlainText = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(token.PlainText))
	token.Hash = hash[:]
	return token, nil
}

func (t *Token) HashToken(plainText string) []byte {
	hash := sha256.Sum256([]byte(plainText))
	return hash[:]
}

// CompareTokenHash verifies whether the given plaintext token matches the stored hash.
func (t *Token) CompareTokenHash(plainTextToken string, storedHash []byte) (bool, error) {
	// Compute the hash of the provided plaintext token
	//computedHash := sha256.Sum256([]byte(plainTextToken))

	// Securely compare the computed hash with the stored hash
	if subtle.ConstantTimeCompare(t.HashToken(plainTextToken), storedHash) == 1 {
		return true, nil
	}
	return false, errors.New("token hash mismatch")
}
