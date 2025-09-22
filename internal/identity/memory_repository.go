package identity

import (
    "context"
    "errors"
    "sync"
)

type memoryRepository struct {
    mu    sync.RWMutex
    users map[string]User
}

// NewMemoryRepository builds an in-memory user store for testing.
func NewMemoryRepository() Repository {
    return &memoryRepository{users: make(map[string]User)}
}

func (r *memoryRepository) Create(_ context.Context, user User) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.users[user.Phone]; exists {
        return errors.New("user exists")
    }
    r.users[user.Phone] = user
    return nil
}

func (r *memoryRepository) FindByPhone(_ context.Context, phone string) (User, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    user, ok := r.users[phone]
    if !ok {
        return User{}, errors.New("user not found")
    }
    return user, nil
}

func (r *memoryRepository) UpdateDevice(_ context.Context, id, deviceID string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    for phone, user := range r.users {
        if user.ID == id {
            user.DeviceID = deviceID
            r.users[phone] = user
            return nil
        }
    }
    return errors.New("user not found")
}
