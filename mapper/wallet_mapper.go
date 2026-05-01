package mapper

import (
	"time"

	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
)

// TopupResultToResponse maps domain top-up outcome to the API response (only fields we expose).
func TopupResultToResponse(r *entity.TopupResult) *dto.TopupResponse {
	if r == nil {
		return nil
	}
	return &dto.TopupResponse{
		ChargeID:  r.ChargeID,
		QRCodeURL: r.QRCodeURL,
		Status:    r.Status,
	}
}

// TopupRequestToInput maps HTTP input plus authenticated user to a domain top-up input.
func TopupRequestToInput(userID string, req *dto.TopupRequest) entity.TopupInput {
	if req == nil {
		return entity.TopupInput{UserID: userID}
	}
	return entity.TopupInput{UserID: userID, Amount: req.Amount}
}

// TransactionToItem maps a stored transaction to an API list item.
func TransactionToItem(t entity.Transaction) dto.TransactionItem {
	return dto.TransactionItem{
		ChargeID:  t.ChargeID,
		Amount:    t.Amount,
		Status:    t.Status,
		Paid:      t.Paid,
		Credited:  t.Credited,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
}

// TransactionsToListResponse maps a slice of transactions to the list API shape.
func TransactionsToListResponse(items []entity.Transaction) dto.TransactionListResponse {
	out := make([]dto.TransactionItem, 0, len(items))
	for _, item := range items {
		out = append(out, TransactionToItem(item))
	}
	return dto.TransactionListResponse{Items: out, Total: len(out)}
}

// WebhookPayloadToCharge extracts charge fields from an Omise webhook JSON object.
// The second return value is false when the event should be ignored (ack 200, no processing).
func WebhookPayloadToCharge(payload map[string]any) (entity.WebhookCharge, bool) {
	var empty entity.WebhookCharge
	if payload == nil {
		return empty, false
	}

	objectType, _ := payload["object"].(string)
	chargeID := ""
	status := ""
	paid := false

	if objectType == "charge" {
		chargeID, _ = payload["id"].(string)
		status, _ = payload["status"].(string)
		paid, _ = payload["paid"].(bool)
	}

	if objectType == "event" {
		if dataMap, ok := payload["data"].(map[string]any); ok {
			if dataObjectType, _ := dataMap["object"].(string); dataObjectType == "charge" {
				objectType = dataObjectType
				chargeID, _ = dataMap["id"].(string)
				status, _ = dataMap["status"].(string)
				paid, _ = dataMap["paid"].(bool)
			} else if objectMap, ok := dataMap["object"].(map[string]any); ok {
				objectType, _ = objectMap["object"].(string)
				chargeID, _ = objectMap["id"].(string)
				status, _ = objectMap["status"].(string)
				paid, _ = objectMap["paid"].(bool)
			}
		}
	}

	if objectType != "charge" || chargeID == "" {
		return empty, false
	}
	return entity.WebhookCharge{ChargeID: chargeID, Status: status, Paid: paid}, true
}
