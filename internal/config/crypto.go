package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/denisbrodbeck/machineid"
)

type Config struct {
	ActiveProvider string `json:"active_provider"`
}

var (
	ConfigDir  string
	ConfigFile string
	SecretFile string
)

func init() {
	userConfig, err := os.UserConfigDir()
	if err != nil {
		userConfig = "."
	}
	ConfigDir = filepath.Join(userConfig, "teragen")
	ConfigFile = filepath.Join(ConfigDir, "config.json")
	SecretFile = filepath.Join(ConfigDir, "secrets.env")

	if _, err := os.Stat(ConfigDir); os.IsNotExist(err) {
		os.MkdirAll(ConfigDir, 0755)
	}
}

func GetHWID() (string, error) {
	id, err := machineid.ID()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(id))
	return hex.EncodeToString(hash[:]), nil
}

func encrypt(plaintext string, key string) (string, error) {
	hash := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func decrypt(ciphertextStr string, key string) (string, error) {
	hash := sha256.Sum256([]byte(key))
	ciphertext, err := hex.DecodeString(ciphertextStr)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
