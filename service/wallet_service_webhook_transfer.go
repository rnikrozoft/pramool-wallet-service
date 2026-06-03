package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rnikrozoft/pramool-wallet-service/internal/omisehttp"
	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
)

// FetchTransferStateFromAPI loads transfer status from Omise (GET /transfers/:id).
func (s *WalletService) FetchTransferStateFromAPI(ctx context.Context, transferID string) (status string, err error) {
	transferID = strings.TrimSpace(transferID)
	if transferID == "" {
		return "", errors.New("empty transfer id")
	}
	if s.omiseSecretKey == "" {
		return "", errors.New("missing omise secret key")
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.omise.co/transfers/%s", url.PathEscape(transferID)), nil)
	if err != nil {
		return "", err
	}
	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise transfer fetch failed")
	if err != nil {
		return "", err
	}
	var tr struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", err
	}
	return strings.TrimSpace(tr.Status), nil
}

// ProcessWebhookTransfer updates withdrawal rows from Omise transfer.* webhook events.
func (s *WalletService) ProcessWebhookTransfer(ctx context.Context, in entity.WebhookTransfer) error {
	transferID := strings.TrimSpace(in.TransferID)
	if transferID == "" {
		return nil
	}
	key := strings.TrimSpace(in.EventKey)
	omiseStatus := strings.TrimSpace(in.Status)

	newStatus, ok := entity.WithdrawStatusFromWebhookEvent(key, omiseStatus)
	if !ok {
		return nil
	}

	if newStatus == entity.WithdrawStatusFailed {
		reason := "omise transfer failed"
		switch key {
		case "transfer.destroy":
			reason = "omise transfer cancelled"
		default:
			if omiseStatus != "" {
				reason = "omise transfer failed: " + omiseStatus
			}
		}
		return s.repository.FailWithdrawalByTransferID(ctx, transferID, reason)
	}

	_, err := s.repository.UpdateWithdrawalStatusByTransfer(ctx, transferID, newStatus, "")
	return err
}
