package entity

// TopupStatusResult is the synced Omise charge state for an in-progress top-up.
type TopupStatusResult struct {
	ChargeID     string
	QRCodeURL    string
	Status       string
	Paid         bool
	Credited     bool
	Expired      bool
	ExpiresAt    string
	DisputeStatus string
	PaidAmount   int64
	FeeAmount    int64
	CreditAmount int64
}
