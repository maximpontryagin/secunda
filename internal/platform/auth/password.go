package auth

import "golang.org/x/crypto/bcrypt"

type Passwords struct{ dummyHash string }

func NewPasswords() (*Passwords, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("unused-password-for-timing"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return &Passwords{dummyHash: string(dummy)}, nil
}

func (p *Passwords) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (p *Passwords) Compare(hash, password string) error {
	if hash == "" {
		hash = p.dummyHash
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
