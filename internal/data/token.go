package data

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"time"
)

const TokenSize = 16

type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	Expiry    time.Time `json:"expiry"`
}

func generateToken(ttl time.Duration) (*Token, error) {
	randomBytes := make([]byte, TokenSize)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	str := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	t := &Token{
		Plaintext: str,
		Hash:      generateHash(str),
		Expiry:    time.Now().Add(ttl),
	}

	return t, nil
}

func generateHash(str string) []byte {
	hash := sha256.Sum256([]byte(str))

	return hash[:]
}
