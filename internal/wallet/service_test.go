package wallet

import (
    "context"
    "testing"

    "github.com/google/uuid"

    "github.com/congo-pay/congo_pay/internal/ledger"
)

func TestServiceCreateAndBalance(t *testing.T) {
    repo := NewMemoryRepository()
    led := ledger.NewInMemory()
    svc := NewService(repo, led)

    ctx := context.Background()
    ownerID := uuid.NewString()
    wallet, err := svc.Create(ctx, CreateInput{OwnerID: ownerID, Currency: "XAF"})
    if err != nil {
        t.Fatalf("create wallet: %v", err)
    }

    fetched, err := svc.Get(ctx, wallet.ID)
    if err != nil {
        t.Fatalf("get wallet: %v", err)
    }
    if fetched.ID != wallet.ID || fetched.OwnerID != ownerID {
        t.Fatalf("expected wallet ID %s, got %s", wallet.ID, fetched.ID)
    }

    ledger.SeedBalance(led, wallet.AccountCode, 2_500)

    balance, err := svc.Balance(ctx, wallet.ID)
    if err != nil {
        t.Fatalf("balance: %v", err)
    }
    if balance.Amount != 2_500 {
        t.Fatalf("expected balance 2500, got %d", balance.Amount)
    }
}
