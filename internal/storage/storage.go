package storage

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

type AccountData struct {
	Address   string `json:"address"`
	Password  string `json:"password"`
	Token     string `json:"token"`
	AccountID string `json:"accountId"`
	CreatedAt string `json:"createdAt"`
}

const (
	keyringService = "burnmail"
	keyringUser    = "default"
)

var (
	configPath     string
	configPathOnce sync.Once
	configPathErr  error
)

func getConfigPath() (string, error) {
	configPathOnce.Do(func() {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			configPathErr = err
			return
		}
		configPath = filepath.Join(homeDir, ".burnmail.json")
	})
	return configPath, configPathErr
}

func getOrCreatePassword() (string, error) {
	password, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			return "", err
		}
		password = base64.StdEncoding.EncodeToString(randomBytes)

		if err := keyring.Set(keyringService, keyringUser, password); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	return password, nil
}

func Save(data *AccountData) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	password, err := getOrCreatePassword()
	if err != nil {
		// Keyring unavailable (e.g. headless session): fall back to plaintext
		// so the tool keeps working, but make the downgrade visible.
		fmt.Fprintf(os.Stderr, "warning: keyring unavailable (%v), storing account data unencrypted\n", err)
		return os.WriteFile(path, jsonData, 0600)
	}

	encrypted, err := Encrypt(jsonData, password)
	if err != nil {
		return err
	}

	return os.WriteFile(path, encrypted, 0600)
}

func Load() (*AccountData, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	password, err := getOrCreatePassword()
	if err == nil {
		decrypted, err := Decrypt(data, password)
		if err == nil {
			data = decrypted
		}
	}

	var account AccountData
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

func Delete() error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	_ = keyring.Delete(keyringService, keyringUser)

	return nil
}

func Exists() bool {
	path, err := getConfigPath()
	if err != nil {
		return false
	}

	_, err = os.Stat(path)
	return err == nil
}
