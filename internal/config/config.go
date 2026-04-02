// Package config manages application settings and multi-account storage.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const appName = "tgmsgcleaner"

type GlobalConfig struct {
	APIID   int    `json:"api_id"`
	APIHash string `json:"api_hash"`
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", appName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

func LoadGlobal() (GlobalConfig, error) {
	dir, err := Dir()
	if err != nil {
		return GlobalConfig{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return GlobalConfig{}, err
	}
	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return GlobalConfig{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func SaveGlobal(cfg GlobalConfig) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0600)
}

func ListAccounts() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	accountsDir := filepath.Join(dir, "accounts")
	entries, err := os.ReadDir(accountsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read accounts dir: %w", err)
	}
	var phones []string
	for _, e := range entries {
		if e.IsDir() {
			phones = append(phones, e.Name())
		}
	}
	sort.Strings(phones)
	return phones, nil
}

func SessionPath(phone string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	accountDir := filepath.Join(dir, "accounts", phone)
	if err := os.MkdirAll(accountDir, 0700); err != nil {
		return "", fmt.Errorf("create account dir: %w", err)
	}
	return filepath.Join(accountDir, "session.json"), nil
}

func Reset() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}
