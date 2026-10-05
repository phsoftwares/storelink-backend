package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func GenerateAPIKey() (string, string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", err
	}
	key := hex.EncodeToString(random)
	return key, HashAPIKey(key), nil
}

func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func CompareAPIKey(storedHash, providedKey string) bool {
	if strings.HasPrefix(storedHash, "$2a$") || strings.HasPrefix(storedHash, "$2b$") || strings.HasPrefix(storedHash, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(providedKey)) == nil
	}
	stored, err := hex.DecodeString(storedHash)
	if err != nil || len(stored) != sha256.Size {
		return false
	}
	provided := sha256.Sum256([]byte(providedKey))
	return subtle.ConstantTimeCompare(stored, provided[:]) == 1
}
