package securestore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var (
	keyOnce sync.Once
	keyData []byte
	keyErr  error
	keySrc  string
)

func loadKey() ([]byte, error) {
	keyOnce.Do(func() {
		if envKey := os.Getenv("APP_SECRET_KEY"); envKey != "" {
			decoded, err := base64.StdEncoding.DecodeString(envKey)
			if err == nil && len(decoded) == 32 {
				keyData = decoded
				keySrc = "env:APP_SECRET_KEY(base64)"
				return
			}

			if len(envKey) == 32 {
				keyData = []byte(envKey)
				keySrc = "env:APP_SECRET_KEY(raw)"
				return
			}

			keyErr = errors.New("APP_SECRET_KEY must be 32 bytes raw or base64-encoded 32 bytes")
			return
		}

		keyPath := os.Getenv("APP_SECRET_KEY_FILE")
		if keyPath == "" {
			keyPath = ".app_secret"
		}

		keyPath = filepath.Clean(keyPath)
		content, err := os.ReadFile(keyPath)
		if err == nil {
			decoded, decodeErr := base64.StdEncoding.DecodeString(string(content))
			if decodeErr == nil && len(decoded) == 32 {
				keyData = decoded
				keySrc = "file:" + keyPath
				return
			}
			keyErr = errors.New("invalid secret key file contents")
			return
		}

		if !errors.Is(err, os.ErrNotExist) {
			keyErr = err
			return
		}

		generated := make([]byte, 32)
		if _, err := rand.Read(generated); err != nil {
			keyErr = err
			return
		}

		encoded := base64.StdEncoding.EncodeToString(generated)
		if writeErr := os.WriteFile(keyPath, []byte(encoded), 0600); writeErr != nil {
			keyErr = writeErr
			return
		}

		keyData = generated
		keySrc = "generated:file:" + keyPath
	})

	return keyData, keyErr
}

func EncryptString(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}

	key, err := loadKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// EncryptBytes encrypts arbitrary byte payloads with AES-GCM.
func EncryptBytes(plain []byte) ([]byte, error) {
	key, err := loadKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plain, nil), nil
}

// DecryptBytes decrypts byte payloads previously encrypted by EncryptBytes.
func DecryptBytes(cipherPayload []byte) ([]byte, error) {
	key, err := loadKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(cipherPayload) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := cipherPayload[:gcm.NonceSize()]
	message := cipherPayload[gcm.NonceSize():]
	return gcm.Open(nil, nonce, message, nil)
}

func DecryptString(cipherText string) (string, error) {
	if cipherText == "" {
		return "", nil
	}

	key, err := loadKey()
	if err != nil {
		return "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(decoded) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce := decoded[:gcm.NonceSize()]
	message := decoded[gcm.NonceSize():]
	plainText, err := gcm.Open(nil, nonce, message, nil)
	if err != nil {
		return "", err
	}

	return string(plainText), nil
}

func KeyBase64() (string, error) {
	key, err := loadKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func KeyFingerprint() (string, error) {
	key, err := loadKey()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:8]), nil
}

func KeySource() string {
	_, _ = loadKey()
	if keySrc == "" {
		return "unknown"
	}
	return keySrc
}
