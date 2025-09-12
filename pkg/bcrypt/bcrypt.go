package bcrypt

import "golang.org/x/crypto/bcrypt"

type Encryptor interface {
	CompareHashAndPassword(hash, password []byte) error
	GenerateFromPassword(password []byte) ([]byte, error)
}

type Encrypt struct {
}

func NewEncrypt() *Encrypt {
	return &Encrypt{}
}

func (b *Encrypt) CompareHashAndPassword(hashPassword []byte, password []byte) error {
	return bcrypt.CompareHashAndPassword(hashPassword, password)
}

func (b *Encrypt) GenerateFromPassword(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
}
