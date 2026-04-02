package keyring

import (
	"fmt"
	"sync"
)

// Mock is an in-memory Keyring implementation for tests.
type Mock struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewMock() *Mock {
	return &Mock{data: make(map[string]string)}
}

func (m *Mock) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *Mock) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrNotFound, key)
	}
	return v, nil
}

func (m *Mock) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}
