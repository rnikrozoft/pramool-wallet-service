package dto

// TopupResponse is returned after creating an Omise PromptPay charge.
type TopupResponse struct {
	ChargeID  string `json:"charge_id"`
	QRCodeURL string `json:"qr_code_url"`
	Status    string `json:"status"`
}

// TransactionItem is one row in GET /wallet/transactions.
type TransactionItem struct {
	ChargeID  string `json:"charge_id"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
	Paid      bool   `json:"paid"`
	Credited  bool   `json:"credited"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TransactionListResponse is the JSON body for GET /wallet/transactions.
type TransactionListResponse struct {
	Items []TransactionItem `json:"items"`
	Total int               `json:"total"`
}
