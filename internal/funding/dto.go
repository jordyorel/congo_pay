package funding

// CardInRequest captures user-provided data to fund a wallet from a card.
type CardInRequest struct {
	CardNumber string `json:"card_number"`
	Expiry     string `json:"expiry"`
	CVV        string `json:"cvv"`
	Amount     int64  `json:"amount_cfa"`
	ClientTxID string `json:"client_tx_id"`
}

// CardOutRequest captures withdrawal details to push funds to a card.
type CardOutRequest struct {
	CardNumber string `json:"card_number"`
	Amount     int64  `json:"amount_cfa"`
	ClientTxID string `json:"client_tx_id"`
}

// FundingResponse represents the API response for card funding actions.
type FundingResponse struct {
	TransactionID     string `json:"transaction_id"`
	Status            string `json:"status"`
	WalletBalance     int64  `json:"wallet_balance_cfa"`
	AcquirerReference string `json:"acquirer_reference"`
}
