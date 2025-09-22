package ledger

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestInMemoryLedger_TransferMaintainsBalance(t *testing.T) {
	l := NewInMemory()
	ctx := context.Background()

	if err := l.EnsureAccount(ctx, "wallet:a"); err != nil {
		t.Fatalf("ensure account a: %v", err)
	}
	if err := l.EnsureAccount(ctx, "wallet:b"); err != nil {
		t.Fatalf("ensure account b: %v", err)
	}

	// seed account a with funds via manual mutation (test helper)
	SeedBalance(l, "wallet:a", 10_000)

	res, err := l.Transfer(ctx, "wallet:a", "wallet:b", "p2p", "client-1", 1_500)
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	if res.FromBalance != 8_500 {
		t.Fatalf("expected from balance 8500, got %d", res.FromBalance)
	}
	if res.ToBalance != 1_500 {
		t.Fatalf("expected to balance 1500, got %d", res.ToBalance)
	}

	ledgerImpl := l.(*inMemoryLedger)
	total := ledgerImpl.balances["wallet:a"] + ledgerImpl.balances["wallet:b"]
	if total != 10_000 {
		t.Fatalf("ledger not balanced, total=%d", total)
	}
}

func TestInMemoryLedger_DuplicateTransaction(t *testing.T) {
	l := NewInMemory()
	ctx := context.Background()
	l.EnsureAccount(ctx, "wallet:a")
	l.EnsureAccount(ctx, "wallet:b")
	SeedBalance(l, "wallet:a", 5_000)

	if _, err := l.Transfer(ctx, "wallet:a", "wallet:b", "p2p", "dup", 500); err != nil {
		t.Fatalf("initial transfer failed: %v", err)
	}
	if _, err := l.Transfer(ctx, "wallet:a", "wallet:b", "p2p", "dup", 500); err != ErrDuplicateTransaction {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestInMemoryLedger_ConcurrentTransfers(t *testing.T) {
	l := NewInMemory()
	ctx := context.Background()
	l.EnsureAccount(ctx, "wallet:a")
	l.EnsureAccount(ctx, "wallet:b")
	SeedBalance(l, "wallet:a", 100_000)
	ledgerImpl := l.(*inMemoryLedger)

	const workers = 10
	const amount = int64(500)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			txID := fmt.Sprintf("tx-%d", i)
			if _, err := l.Transfer(ctx, "wallet:a", "wallet:b", "p2p", txID, amount); err != nil {
				t.Errorf("transfer %d failed: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	total := ledgerImpl.balances["wallet:a"] + ledgerImpl.balances["wallet:b"]
	if total != 100_000 {
		t.Fatalf("ledger not balanced after concurrency, total=%d", total)
	}
}

func TestInMemoryLedger_CardIn(t *testing.T) {
	l := NewInMemory()
	ctx := context.Background()
	l.EnsureAccount(ctx, "wallet:a")
	l.EnsureAccount(ctx, CardSuspenseAccountCode)

	res, err := l.CardIn(ctx, "wallet:a", "client-card-in", 2_000)
	if err != nil {
		t.Fatalf("card in failed: %v", err)
	}
	if res.Status != FundingStatusPendingSettlement {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	if res.WalletBalance != 2_000 {
		t.Fatalf("expected wallet balance 2000, got %d", res.WalletBalance)
	}

	if _, err := l.CardIn(ctx, "wallet:a", "client-card-in", 2_000); err != ErrDuplicateTransaction {
		t.Fatalf("expected duplicate card in error, got %v", err)
	}
}

func TestInMemoryLedger_CardOut(t *testing.T) {
	l := NewInMemory()
	ctx := context.Background()
	l.EnsureAccount(ctx, "wallet:a")
	l.EnsureAccount(ctx, CardSuspenseAccountCode)
	SeedBalance(l, "wallet:a", 5_000)

	res, err := l.CardOut(ctx, "wallet:a", "client-card-out", 1_500)
	if err != nil {
		t.Fatalf("card out failed: %v", err)
	}
	if res.WalletBalance != 3_500 {
		t.Fatalf("expected wallet balance 3500, got %d", res.WalletBalance)
	}

	if _, err := l.CardOut(ctx, "wallet:a", "client-card-out", 1_500); err != ErrDuplicateTransaction {
		t.Fatalf("expected duplicate card out error, got %v", err)
	}

	if _, err := l.CardOut(ctx, "wallet:a", "client-card-out-2", 10_000); err != ErrInsufficientFunds {
		t.Fatalf("expected insufficient funds, got %v", err)
	}
}
