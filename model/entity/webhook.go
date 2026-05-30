package entity

// WebhookCharge is normalized Omise charge data extracted from a webhook payload.
type WebhookCharge struct {
	ChargeID string
	Status   string
	Paid     bool
}

// WebhookTransfer is normalized Omise transfer data from a webhook event.
type WebhookTransfer struct {
	TransferID string
	Status     string
	EventKey   string
}
