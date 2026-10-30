package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const settingsFile = "settings.dat"

// Store persists Settings in the user's config directory. The file is
// encrypted with the OS user key so other accounts cannot read secrets.
type Store struct {
	mu   sync.Mutex
	path string
}

func DataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "WDTT Plus")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func NewStore(dir string) *Store {
	return &Store{path: filepath.Join(dir, settingsFile)}
}

func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings := DefaultSettings()
	sealed, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	plain, err := unprotect(sealed)
	if err != nil {
		return settings, fmt.Errorf("не удалось расшифровать настройки: %w", err)
	}
	if err := json.Unmarshal(plain, &settings); err != nil {
		return settings, fmt.Errorf("настройки повреждены: %w", err)
	}
	settings.Normalize()
	return settings, nil
}

func (s *Store) Save(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	plain, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	sealed, err := protect(plain)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, sealed, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
