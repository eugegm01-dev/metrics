package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
)

// Encrypt шифрует данные с использованием публичного RSA-ключа.
func Encrypt(data []byte, pubKey *rsa.PublicKey) ([]byte, error) {
	if pubKey == nil {
		return nil, errors.New("public key is nil")
	}
	// Используем OAEP с SHA-256
	hash := sha256.New()
	return rsa.EncryptOAEP(hash, rand.Reader, pubKey, data, nil)
}

// Decrypt расшифровывает данные с использованием приватного RSA-ключа.
func Decrypt(ciphertext []byte, privKey *rsa.PrivateKey) ([]byte, error) {
	if privKey == nil {
		return nil, errors.New("private key is nil")
	}
	hash := sha256.New()
	return rsa.DecryptOAEP(hash, rand.Reader, privKey, ciphertext, nil)
}
