package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rnikrozoft/pramool-wallet-service/internal/omisehttp"
	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
)

// ProcessWebhookDispute applies Omise dispute lifecycle to a stored top-up transaction.
func (s *WalletService) ProcessWebhookDispute(_ context.Context, d entity.WebhookDispute) error {
	d.DisputeID = strings.TrimSpace(d.DisputeID)
	d.ChargeID = strings.TrimSpace(d.ChargeID)
	d.Status = strings.TrimSpace(d.Status)
	d.EventKey = strings.TrimSpace(d.EventKey)
	if d.ChargeID == "" {
		return nil
	}
	row, err := s.repository.GetTransactionByChargeID(d.ChargeID)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}

	if d.DisputeID != "" {
		if err := s.repository.UpdateTransactionDispute(d.ChargeID, d.DisputeID, d.Status); err != nil {
			return err
		}
	}

	switch {
	case d.EventKey == "dispute.accept" || strings.EqualFold(d.Status, "lost"):
		return s.reverseTopupCreditForDispute(d.ChargeID)
	case strings.EqualFold(d.Status, "won"):
		return s.repository.MarkTopupDisputeResolved(d.ChargeID, "won", "successful")
	case strings.EqualFold(d.Status, "open"), strings.EqualFold(d.Status, "pending"):
		paid := row.Paid
		if paid {
			return s.repository.UpdateTransactionStatus(d.ChargeID, "disputed", true)
		}
		return s.repository.UpdateTransactionStatus(d.ChargeID, "disputed", false)
	default:
		return nil
	}
}

func (s *WalletService) reverseTopupCreditForDispute(chargeID string) error {
	userID, credit, claimed, err := s.repository.ClaimTopupCreditReversal(chargeID)
	if err != nil {
		return err
	}
	if !claimed || credit <= 0 {
		return s.repository.MarkTopupDisputeResolved(chargeID, "lost", "dispute_lost")
	}
	if err := s.repository.DeductUserCredit(userID, credit); err != nil {
		return err
	}
	return nil
}

// FetchDisputeStateFromAPI loads dispute status and linked charge from Omise (GET /disputes/:id).
func (s *WalletService) FetchDisputeStateFromAPI(ctx context.Context, disputeID string) (status, chargeID string, err error) {
	disputeID = strings.TrimSpace(disputeID)
	if disputeID == "" {
		return "", "", fmt.Errorf("empty dispute id")
	}
	if s.omiseSecretKey == "" {
		return "", "", fmt.Errorf("missing omise secret key")
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.omise.co/disputes/%s", disputeID), nil)
	if err != nil {
		return "", "", err
	}
	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise dispute fetch failed")
	if err != nil {
		return "", "", err
	}
	var ds struct {
		Status string `json:"status"`
		Charge any    `json:"charge"`
	}
	if err := json.Unmarshal(body, &ds); err != nil {
		return "", "", err
	}
	return strings.TrimSpace(ds.Status), chargeIDFromOmiseField(ds.Charge), nil
}

func chargeIDFromOmiseField(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		id, _ := t["id"].(string)
		return strings.TrimSpace(id)
	default:
		return ""
	}
}

func (s *WalletService) applyChargeDisputeFromDetails(chargeID string, details omiseChargeDetails) error {
	if details.DisputeID == "" && details.DisputeStatus == "" {
		return nil
	}
	if err := s.repository.UpdateTransactionDispute(chargeID, details.DisputeID, details.DisputeStatus); err != nil {
		return err
	}
	return s.ProcessWebhookDispute(context.Background(), entity.WebhookDispute{
		DisputeID: details.DisputeID,
		ChargeID:  chargeID,
		Status:    details.DisputeStatus,
		EventKey:  disputeEventKeyForStatus(details.DisputeStatus),
	})
}

func disputeEventKeyForStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "won", "lost":
		return "dispute.close"
	case "pending":
		return "dispute.update"
	default:
		return "dispute.create"
	}
}
