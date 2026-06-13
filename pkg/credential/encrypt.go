package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/yi-nology/git-platform-sdk/pkg/encoding"
)

var (
	key     []byte
	keyOnce sync.Once
)

func getKey() ([]byte, error) {
	var onceErr error
	keyOnce.Do(func() {
		k := os.Getenv("ENCRYPTION_KEY")
		if k == "" {
			onceErr = fmt.Errorf("ENCRYPTION_KEY environment variable is required for credential encryption")
			return
		}
		if len(k) != 32 {
			onceErr = fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes, got %d", len(k))
			return
		}
		key = []byte(k)
	})
	if onceErr != nil {
		return nil, onceErr
	}
	return key, nil
}

func EncryptGCM(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	k, err := getKey()
	if err != nil {
		return "", err
	}

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

func DecryptGCM(cryptoText string) (string, error) {
	if cryptoText == "" {
		return "", nil
	}

	k, err := getKey()
	if err != nil {
		return "", err
	}

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
