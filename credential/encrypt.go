package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"sync"

	"git.enjoye.top/enjoydream/ekit/pkg/encoding"
)

var (
	key     []byte
	keyOnce sync.Once
)

func getKey() []byte {
	keyOnce.Do(func() {
		k := os.Getenv("ENCRYPTION_KEY")
		if k == "" {
			k = "12345678901234567890123456789012"
		}
		key = []byte(k)
	})
	return key
}

func EncryptGCM(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(getKey())
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

	dataStr, err := encoding.Base64URLDecode(cryptoText)
	if err != nil {
		return "", err
	}
	data := []byte(dataStr)

	block, err := aes.NewCipher(getKey())
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
