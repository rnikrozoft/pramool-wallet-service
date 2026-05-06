package dto

// TopupRequest is the JSON body for POST /wallet/topup.
type TopupRequest struct {
	Amount int64 `json:"amount"`
}
