package entity

// TopupInput is the domain input for creating a PromptPay top-up (after mapping from HTTP).
type TopupInput struct {
	UserID string
	Amount int64
}

// TopupResult is the outcome of creating a PromptPay charge (domain layer; map to dto in handler).
type TopupResult struct {
	ChargeID  string
	QRCodeURL string
	Status    string
}
