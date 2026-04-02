package keyring

import (
	"errors"
	"fmt"

	gokeyring "github.com/zalando/go-keyring"
)

const serviceName = "pcurl"

// Keyring abstracts OS keychain operations.
type Keyring interface {
	Set(key, value string) error
	Get(key string) (string, error)
	Delete(key string) error
}

// OS implements Keyring via zalando/go-keyring.
type OS struct{}

func (k *OS) Set(key, value string) error {
	return gokeyring.Set(serviceName, key, value)
}

func (k *OS) Get(key string) (string, error) {
	v, err := gokeyring.Get(serviceName, key)
	if err != nil {
		return "", fmt.Errorf("keyring: get %q: %w", key, err)
	}
	return v, nil
}

func (k *OS) Delete(key string) error {
	if err := gokeyring.Delete(serviceName, key); err != nil {
		return fmt.Errorf("keyring: delete %q: %w", key, err)
	}
	return nil
}

// ErrNotFound is returned when a key is not found.
var ErrNotFound = errors.New("keyring: key not found")
