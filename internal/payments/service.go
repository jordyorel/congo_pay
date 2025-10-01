package payments

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"

    "github.com/congo-pay/congo_pay/internal/ledger"
    "github.com/congo-pay/congo_pay/internal/notification"
    "github.com/congo-pay/congo_pay/internal/wallet"
)

// Service wires wallet ledger postings for P2P transfers.
type Service struct {
    ledger        ledger.Ledger
    walletService *wallet.Service
    notifier      notification.Notifier
}

// NewService constructs a payment service.
func NewService(ledger ledger.Ledger, walletService *wallet.Service, notifier notification.Notifier) *Service {
    return &Service{ledger: ledger, walletService: walletService, notifier: notifier}
}

// TransferInput captures the data needed to move funds between wallets.
type TransferInput struct {
    FromWalletID string
    ToWalletID   string
    Amount       int64
    ClientTxID   string
    RequestorUserID string
}

// TransferResult describes the ledger outcome of a P2P transfer.
type TransferResult struct {
    TransactionID string
    FromBalance   int64
    ToBalance     int64
    CompletedAt   time.Time
}

// ErrNotOwner indicates the caller does not own the source wallet.
var ErrNotOwner = errors.New("not owner of source wallet")

// Transfer posts a balanced ledger entry between two wallets.
func (s *Service) Transfer(ctx context.Context, input TransferInput) (TransferResult, error) {
    if input.Amount <= 0 {
        return TransferResult{}, fmt.Errorf("amount must be positive")
    }
    if input.ClientTxID == "" {
        input.ClientTxID = uuid.New().String()
    }

    fromWallet, err := s.walletService.Get(ctx, input.FromWalletID)
    if err != nil {
        return TransferResult{}, err
    }
    if input.RequestorUserID != "" && fromWallet.OwnerID != input.RequestorUserID {
        return TransferResult{}, ErrNotOwner
    }
    toWallet, err := s.walletService.Get(ctx, input.ToWalletID)
    if err != nil {
        return TransferResult{}, err
    }

    res, err := s.ledger.Transfer(ctx, fromWallet.AccountCode, toWallet.AccountCode, "p2p", input.ClientTxID, input.Amount)
    if err != nil {
        if errors.Is(err, ledger.ErrInsufficientFunds) || errors.Is(err, ledger.ErrDuplicateTransaction) {
            return TransferResult{}, err
        }
        return TransferResult{}, err
    }

    outcome := TransferResult{
        TransactionID: res.TransactionID,
        FromBalance:   res.FromBalance,
        ToBalance:     res.ToBalance,
        CompletedAt:   time.Now().UTC(),
    }

    if s.notifier != nil {
        _ = s.notifier.Send(ctx, notification.Message{
            Kind:        notification.KindP2PTransfer,
            Destination: toWallet.OwnerID,
            Body:        fmt.Sprintf("You received %d from wallet %s", input.Amount, input.FromWalletID),
        })
    }

    return outcome, nil
}
