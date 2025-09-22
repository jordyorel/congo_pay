package wallet

import (
    "context"
    "errors"
    "sync"
)

type memoryRepository struct {
    mu      sync.RWMutex
    storage map[string]Wallet
}

// NewMemoryRepository constructs an in-memory repository for tests.
func NewMemoryRepository() Repository {
    return &memoryRepository{storage: make(map[string]Wallet)}
}

func (r *memoryRepository) Create(_ context.Context, wallet Wallet) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.storage[wallet.ID]; exists {
        return errors.New("wallet exists")
    }
    r.storage[wallet.ID] = wallet
    return nil
}

func (r *memoryRepository) Get(_ context.Context, id string) (Wallet, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    wallet, ok := r.storage[id]
    if !ok {
        return Wallet{}, errors.New("wallet not found")
    }
    return wallet, nil
}
