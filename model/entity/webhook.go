package entity

// WebhookCharge is normalized Omise charge data extracted from a webhook payload.
type WebhookCharge struct {
	ChargeID string
	Status   string
	Paid     bool
}
