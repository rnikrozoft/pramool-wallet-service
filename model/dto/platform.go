package dto

type PlatformRevenueResponse struct {
	TotalPlatformShareBaht  int64 `json:"total_platform_share_baht"`
	CompletedPayoutAuctions int64 `json:"completed_payout_auctions"`
	TotalWithdrawnBaht      int64 `json:"total_withdrawn_baht"`
	AvailableBalanceBaht    int64 `json:"available_balance_baht"`
}

type PlatformWithdrawRequest struct {
	AdminID    int   `json:"admin_id"`
	AmountBaht int64 `json:"amount_baht"`
}

type PlatformWithdrawResponse struct {
	WithdrawalID           int64  `json:"withdrawal_id"`
	AmountBaht             int64  `json:"amount_baht"`
	Status                 string `json:"status"`
	OmiseTransferID        string `json:"omise_transfer_id,omitempty"`
	OmiseStatus            string `json:"omise_status,omitempty"`
	TotalPlatformShareBaht int64  `json:"total_platform_share_baht"`
	TotalWithdrawnBaht     int64  `json:"total_withdrawn_baht"`
	AvailableBalanceBaht   int64  `json:"available_balance_baht"`
}

type PlatformWithdrawListItem struct {
	WithdrawalID    int64  `json:"withdrawal_id"`
	AmountBaht      int64  `json:"amount_baht"`
	Status          string `json:"status"`
	OmiseTransferID string `json:"omise_transfer_id,omitempty"`
	OmiseStatus     string `json:"omise_status,omitempty"`
	CreatedAt       string `json:"created_at"`
}

type PlatformWithdrawListResponse struct {
	Items []PlatformWithdrawListItem `json:"items"`
}
