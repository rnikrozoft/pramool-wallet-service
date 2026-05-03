package dto

// TopupRequest is the JSON body for POST /wallet/topup.
type TopupRequest struct {
	Amount int64 `json:"amount"`
}

type AuctionCloseFeeRequest struct {
	SellerID  string `json:"seller_id"`
	AuctionID string `json:"auction_id"`
	Amount    int64  `json:"amount"`
	// CreditDeduct is THB to take from users.credit now. If nil, defaults to Amount (legacy).
	CreditDeduct *int64 `json:"credit_deduct,omitempty"`
}
