package entity

// WebhookDispute is parsed from Omise dispute.* webhook events.
type WebhookDispute struct {
	DisputeID string
	ChargeID  string
	Status    string
	EventKey  string
}
