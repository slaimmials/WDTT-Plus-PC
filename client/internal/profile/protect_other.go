//go:build !windows

package profile

// Non-Windows builds are used for development and tests only; the settings
// file is still created with owner-only permissions.
func protect(data []byte) ([]byte, error) { return data, nil }

func unprotect(data []byte) ([]byte, error) { return data, nil }
