package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/yi-nology/git-platform-sdk/pkg/encoding"
)

// CryptoManager provides AES-256-GCM encryption/decryption with an
// injectable key source. Use NewCryptoManager or NewCryptoManagerFromKey
// to create an instance. The zero-value CryptoManager is not usable.
type CryptoManager struct {
	mu  sync.RWMutex
	key []byte
}

// NewCryptoManager creates a CryptoManager that reads the encryption key from
// the ENCRYPTION_KEY environment variable. The key must be at least 1 byte;
// keys shorter than 32 bytes are expanded via SHA-256 to produce a 32-byte
// AES-256 key. This allows users to supply passphrases of arbitrary length.
//
// Returns an error if ENCRYPTION_KEY is unset or empty.
func NewCryptoManager() (*CryptoManager, error) {
	k := os.Getenv("ENCRYPTION_KEY")
	if k == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable is required for credential encryption")
	}
	return NewCryptoManagerFromKey(k), nil
}

// NewCryptoManagerFromKey creates a CryptoManager with an explicit key string.
// Keys shorter than 32 bytes are hashed with SHA-256 to derive a 32-byte
// AES-256 key. Keys of exactly 32 bytes are used directly. Keys longer than
// 32 bytes are also hashed to ensure consistent behavior.
func NewCryptoManagerFromKey(key string) *CryptoManager {
	derived := deriveKey([]byte(key))
	return &CryptoManager{key: derived}
}

// deriveKey converts an arbitrary-length key material into a 32-byte AES-256
// key using SHA-256. This allows passphrases of any length to be used safely.
func deriveKey(material []byte) []byte {
	if len(material) == 32 {
		out := make([]byte, 32)
		copy(out, material)
		return out
	}
	hash := sha256.Sum256(material)
	return hash[:]
}

// Encrypt encrypts plaintext using AES-256-GCM. Returns empty string for
// empty input. The nonce is prepended to the ciphertext.
func (cm *CryptoManager) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	cm.mu.RLock()
	k := cm.key
	cm.mu.RUnlock()

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return encoding.Base64URLEncode(string(ciphertext)), nil
}

// Decrypt decrypts ciphertext produced by Encrypt. Returns empty string for
// empty input.
func (cm *CryptoManager) Decrypt(cryptoText string) (string, error) {
	if cryptoText == "" {
		return "", nil
	}

	cm.mu.RLock()
	k := cm.key
	cm.mu.RUnlock()

	dataStr, err := encoding.Base64URLDecode(cryptoText)
	if err != nil {
		return "", err
	}
	data := []byte(dataStr)

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// RotateKey replaces the current encryption key. Subsequent Encrypt calls use
// the new key; Decrypt calls for data encrypted with the old key will fail.
func (cm *CryptoManager) RotateKey(newKey string) {
	cm.mu.Lock()
	cm.key = deriveKey([]byte(newKey))
	cm.mu.Unlock()
}
