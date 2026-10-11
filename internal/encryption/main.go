package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

// Encrypt encrypts plaintext using AES-GCM with the provided key.
// The key must be 16, 24, or 32 bytes long (AES-128, AES-192, or AES-256).
func Encrypt(plaintext []byte) ([]byte, error) {
	decodedKey, _ := hex.DecodeString(key)

	block, err := aes.NewCipher(decodedKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a random nonce of standard size (12 bytes)
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Seal appends the ciphertext and the authentication tag to the nonce prefix.
	// Seal(dst, nonce, plaintext, additionalData)
	return aesGCM.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts a ciphertext prepended with its nonce using AES-GCM.
func Decrypt(ciphertextAndNonce []byte) ([]byte, error) {
	decodedKey, _ := hex.DecodeString(key)

	block, err := aes.NewCipher(decodedKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertextAndNonce) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// Split the nonce prefix from the actual encrypted ciphertext payload
	nonce, ciphertext := ciphertextAndNonce[:nonceSize], ciphertextAndNonce[nonceSize:]

	// Open decrypts and verifies the authentication tag
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err // Fails if the data or nonce has been tampered with
	}

	return plaintext, nil
}
