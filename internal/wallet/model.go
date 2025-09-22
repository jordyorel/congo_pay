package wallet

import "time"

// Wallet represents a stored value account backed by the ledger.
type Wallet struct {
    ID          string
    OwnerID     string
    AccountCode string
    Currency    string
    Status      string
    CreatedAt   time.Time
}

// Balance encapsulates available funds for a wallet.
type Balance struct {
    WalletID string
    Amount   int64
    AsOf     time.Time
}
