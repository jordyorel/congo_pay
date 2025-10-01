package wallet

import (
    "context"
    "errors"
    "sync"
)

type memoryRepository struct {
    mu      sync.RWMutex
    storage map[string]Wallet
    byOwner map[string]string
}

// NewMemoryRepository constructs an in-memory repository for tests.
func NewMemoryRepository() Repository {
    return &memoryRepository{storage: make(map[string]Wallet), byOwner: make(map[string]string)}
}

func (r *memoryRepository) Create(_ context.Context, wallet Wallet) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.storage[wallet.ID]; exists {
        return errors.New("wallet exists")
    }
    r.storage[wallet.ID] = wallet
    if wallet.OwnerID != "" {
        r.byOwner[wallet.OwnerID] = wallet.ID
    }
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

func (r *memoryRepository) FindByOwner(_ context.Context, ownerID string) (Wallet, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    id, ok := r.byOwner[ownerID]
    if !ok {
        return Wallet{}, errors.New("wallet not found")
    }
    w, ok := r.storage[id]
    if !ok {
        return Wallet{}, errors.New("wallet not found")
    }
    return w, nil
}
