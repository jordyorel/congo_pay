package funding

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/congo-pay/congo_pay/internal/ledger"
	"github.com/congo-pay/congo_pay/internal/wallet"
)

func TestServiceCardIn(t *testing.T) {
	ctx := context.Background()
	ledgerBackend := ledger.NewInMemory()
	walletRepo := wallet.NewMemoryRepository()
	walletSvc := wallet.NewService(walletRepo, ledgerBackend)

	ownerID := uuid.NewString()
	walletRec, err := walletSvc.Create(ctx, wallet.CreateInput{OwnerID: ownerID, Currency: "XAF"})
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	service, err := NewService(ctx, ledgerBackend, walletSvc, StaticAcquirer{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	clientTxID := "dup"
	res, err := service.CardIn(ctx, CardInInput{
		WalletID:   walletRec.ID,
		Amount:     10_000,
		CardNumber: "4111111111111111",
		Expiry:     "12/29",
		CVV:        "123",
		ClientTxID: clientTxID,
	})
	if err != nil {
		t.Fatalf("card in: %v", err)
	}
	if res.Status != ledger.FundingStatusPendingSettlement {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	if res.WalletBalance != 10_000 {
		t.Fatalf("expected balance 10000, got %d", res.WalletBalance)
	}

	if _, err := service.CardIn(ctx, CardInInput{
		WalletID:   walletRec.ID,
		Amount:     10_000,
		CardNumber: "4111111111111111",
		ClientTxID: clientTxID,
	}); err == nil {
		t.Fatal("expected duplicate error")
	} else if !errors.Is(err, ledger.ErrDuplicateTransaction) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestServiceCardOut(t *testing.T) {
	ctx := context.Background()
	ledgerBackend := ledger.NewInMemory()
	walletRepo := wallet.NewMemoryRepository()
	walletSvc := wallet.NewService(walletRepo, ledgerBackend)

	ownerID := uuid.NewString()
	walletRec, err := walletSvc.Create(ctx, wallet.CreateInput{OwnerID: ownerID, Currency: "XAF"})
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	ledger.SeedBalance(ledgerBackend, walletRec.AccountCode, 5_000)

	service, err := NewService(ctx, ledgerBackend, walletSvc, StaticAcquirer{})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	res, err := service.CardOut(ctx, CardOutInput{
		WalletID:   walletRec.ID,
		Amount:     2_000,
		CardNumber: "4111111111111111",
	})
	if err != nil {
		t.Fatalf("card out: %v", err)
	}
	if res.WalletBalance != 3_000 {
		t.Fatalf("expected balance 3000, got %d", res.WalletBalance)
	}

	_, err = service.CardOut(ctx, CardOutInput{
		WalletID:   walletRec.ID,
		Amount:     10_000,
		CardNumber: "4111111111111111",
		ClientTxID: "excess",
	})
	if !errors.Is(err, ledger.ErrInsufficientFunds) {
		t.Fatalf("expected insufficient funds, got %v", err)
	}
}
