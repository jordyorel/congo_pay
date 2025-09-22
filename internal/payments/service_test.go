package payments

import (
    "context"
    "testing"

    "github.com/google/uuid"

    "github.com/congo-pay/congo_pay/internal/ledger"
    "github.com/congo-pay/congo_pay/internal/notification"
    "github.com/congo-pay/congo_pay/internal/wallet"
)

type testNotifier struct {
    last notification.Message
}

func (n *testNotifier) Send(_ context.Context, msg notification.Message) error {
    n.last = msg
    return nil
}

func TestTransferSuccess(t *testing.T) {
    led := ledger.NewInMemory()
    repo := wallet.NewMemoryRepository()
    walletSvc := wallet.NewService(repo, led)
    notifier := &testNotifier{}
    svc := NewService(led, walletSvc, notifier)

    ctx := context.Background()
    from, _ := walletSvc.Create(ctx, wallet.CreateInput{OwnerID: uuid.NewString(), Currency: "XAF"})
    to, _ := walletSvc.Create(ctx, wallet.CreateInput{OwnerID: uuid.NewString(), Currency: "XAF"})

    ledger.SeedBalance(led, from.AccountCode, 10_000)

    res, err := svc.Transfer(ctx, TransferInput{FromWalletID: from.ID, ToWalletID: to.ID, Amount: 2_000, ClientTxID: "abc"})
    if err != nil {
        t.Fatalf("transfer failed: %v", err)
    }

    if res.FromBalance != 8_000 || res.ToBalance != 2_000 {
        t.Fatalf("unexpected balances: %+v", res)
    }

    if notifier.last.Kind != notification.KindP2PTransfer {
        t.Fatalf("expected notification to be sent")
    }
}

func TestTransferInsufficientFunds(t *testing.T) {
    led := ledger.NewInMemory()
    repo := wallet.NewMemoryRepository()
    walletSvc := wallet.NewService(repo, led)
    svc := NewService(led, walletSvc, nil)

    ctx := context.Background()
    from, _ := walletSvc.Create(ctx, wallet.CreateInput{OwnerID: uuid.NewString(), Currency: "XAF"})
    to, _ := walletSvc.Create(ctx, wallet.CreateInput{OwnerID: uuid.NewString(), Currency: "XAF"})

    if _, err := svc.Transfer(ctx, TransferInput{FromWalletID: from.ID, ToWalletID: to.ID, Amount: 1_000, ClientTxID: "abc"}); err != ledger.ErrInsufficientFunds {
        t.Fatalf("expected insufficient funds, got %v", err)
    }
}
