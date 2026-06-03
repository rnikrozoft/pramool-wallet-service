package dto

// TopupResponse is returned after creating an Omise PromptPay charge.
type TopupResponse struct {
	ChargeID     string `json:"charge_id"`
	QRCodeURL    string `json:"qr_code_url"`
	Status       string `json:"status"`
	PaidAmount   int64  `json:"paid_amount"`
	FeeAmount    int64  `json:"fee_amount"`
	CreditAmount int64  `json:"credit_amount"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Resumed      bool   `json:"resumed,omitempty"`
}

// TopupStatusResponse is returned by GET /wallet/topup/status after syncing with Omise.
type TopupStatusResponse struct {
	ChargeID     string `json:"charge_id"`
	QRCodeURL    string `json:"qr_code_url,omitempty"`
	Status       string `json:"status"`
	Paid         bool   `json:"paid"`
	Credited     bool   `json:"credited"`
	Expired      bool   `json:"expired"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	DisputeStatus string `json:"dispute_status,omitempty"`
	PaidAmount   int64  `json:"paid_amount"`
	FeeAmount    int64  `json:"fee_amount"`
	CreditAmount int64  `json:"credit_amount"`
}

// FeeRatesResponse documents user-borne Omise fees and platform commission (for UI).
type FeeRatesResponse struct {
	MinTopupGrossTHB           int64   `json:"min_topup_gross_thb"`
	MinWithdrawCreditTHB       int64   `json:"min_withdraw_credit_thb"`
	TopupFeePercent            float64 `json:"topup_fee_percent"`
	TopupFeePPM                int64   `json:"topup_fee_ppm"`
	WithdrawFeeTHB             int64   `json:"withdraw_fee_thb"`
	AuctionFeeNormalPct        int64   `json:"auction_fee_normal_pct"`
	AuctionFeeEarlyPct         int64   `json:"auction_fee_early_pct"`
	AuctionSellerKeepNormalPct int64   `json:"auction_seller_keep_normal_pct"`
	AuctionSellerKeepEarlyPct  int64   `json:"auction_seller_keep_early_pct"`
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
	TopupAmount   *int64 `json:"topup_amount,omitempty"`
	TopupPaid     *int64 `json:"topup_paid,omitempty"`
	TopupFee      *int64 `json:"topup_fee,omitempty"`
	Status       *string `json:"status,omitempty"`
	Paid         *bool   `json:"paid,omitempty"`
	Credited     *bool   `json:"credited,omitempty"`
	BidTxID      *int64  `json:"bid_tx_id,omitempty"`
	AuctionID           *string `json:"auction_id,omitempty"`
	AuctionTitle        *string `json:"auction_title,omitempty"`
	AuctionCoverImageURL *string `json:"auction_cover_image_url,omitempty"`
	LedgerAmount        *int64  `json:"ledger_amount,omitempty"`
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

// WithdrawResponse is returned after POST /wallet/withdraw.
type WithdrawResponse struct {
	WithdrawalID      int64  `json:"withdrawal_id"`
	Amount            int64  `json:"amount"`
	FeeAmount         int64  `json:"fee_amount"`
	TransferAmount    int64  `json:"transfer_amount"`
	Status            string `json:"status"`
	StatusLabel       string `json:"status_label"`
	OmiseTransferID   string `json:"omise_transfer_id,omitempty"`
	BalanceAfter      int64  `json:"balance_after"`
	BankAccountName   string `json:"bank_account_name"`
	BankAccountNumber string `json:"bank_account_number"`
	BankCode          string `json:"bank_code"`
}
