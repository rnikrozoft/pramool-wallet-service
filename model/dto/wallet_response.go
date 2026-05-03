package dto

// TopupResponse is returned after creating an Omise PromptPay charge.
type TopupResponse struct {
	ChargeID  string `json:"charge_id"`
	QRCodeURL string `json:"qr_code_url"`
	Status    string `json:"status"`
}

// TransactionItem is legacy Omise top-up fields (subset of CreditActivityItem).
type TransactionItem struct {
	ChargeID  string `json:"charge_id"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
	Paid      bool   `json:"paid"`
	Credited  bool   `json:"credited"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreditActivityItem is one row in GET /wallet/transactions (topups + auction ledger).
type CreditActivityItem struct {
	EntryType    string  `json:"entry_type"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
	ChargeID     *string `json:"charge_id,omitempty"`
	TopupAmount  *int64  `json:"topup_amount,omitempty"`
	Status       *string `json:"status,omitempty"`
	Paid         *bool   `json:"paid,omitempty"`
	Credited     *bool   `json:"credited,omitempty"`
	BidTxID      *int64  `json:"bid_tx_id,omitempty"`
	AuctionID    *string `json:"auction_id,omitempty"`
	AuctionTitle *string `json:"auction_title,omitempty"`
	LedgerAmount *int64  `json:"ledger_amount,omitempty"`
	BidAmount    *int64  `json:"bid_amount,omitempty"`
	Note         *string `json:"note,omitempty"`
}

// CreditActivityListResponse is the JSON body for GET /wallet/transactions.
type CreditActivityListResponse struct {
	Items  []CreditActivityItem `json:"items"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}
