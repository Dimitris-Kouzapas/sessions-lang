package api

import (
	"fmt"
	"sync"
)

type API interface {
	Request(string) string
	String() string
}

type apiFactory func() API

var (
	mu sync.RWMutex
	apiRegistry = map[string]apiFactory {}
)

func Register(name string, f apiFactory) error {
	mu.Lock()
	defer mu.Unlock()

	if name == "" {
		return fmt.Errorf("register api: empty name")
	}

	if f == nil {
		return fmt.Errorf("register api: nil factory")
	}

	if _, exists := apiRegistry[name]; exists {
		return fmt.Errorf("register api: duplicate api name")
	}

	apiRegistry[name] = f
	return nil
}

func GetAPI(name string) (API, error) {
	mu.RLock()
	f, ok := apiRegistry[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("No api with name %v", name)
	}
	return f(), nil
}

