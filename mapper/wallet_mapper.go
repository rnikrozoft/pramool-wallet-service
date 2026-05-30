package mapper

import (
	"database/sql"
	"strings"
	"time"

	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
)

func ptrSQLString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func ptrSQLInt64(ns sql.NullInt64) *int64 {
	if !ns.Valid {
		return nil
	}
	v := ns.Int64
	return &v
}

func ptrSQLBool(ns sql.NullBool) *bool {
	if !ns.Valid {
		return nil
	}
	b := ns.Bool
	return &b
}

func CreditActivityRowToItem(row entity.CreditActivityRow) dto.CreditActivityItem {
	item := dto.CreditActivityItem{
		EntryType: row.EntryType,
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
	}
	if row.UpdatedAt.Valid {
		item.UpdatedAt = row.UpdatedAt.Time.Format(time.RFC3339)
	}
	item.ChargeID = ptrSQLString(row.ChargeID)
	item.TopupAmount = ptrSQLInt64(row.TopupAmount)
	item.TopupPaid = ptrSQLInt64(row.TopupPaid)
	item.TopupFee = ptrSQLInt64(row.TopupFee)
	item.Status = ptrSQLString(row.Status)
	item.Paid = ptrSQLBool(row.Paid)
	item.Credited = ptrSQLBool(row.Credited)
	item.BidTxID = ptrSQLInt64(row.BidTxID)
	item.AuctionID = ptrSQLString(row.AuctionID)
	item.AuctionTitle = ptrSQLString(row.AuctionTitle)
	item.AuctionCoverImageURL = ptrSQLString(row.AuctionCoverImageURL)
	item.LedgerAmount = ptrSQLInt64(row.LedgerAmount)
	item.BidAmount = ptrSQLInt64(row.BidAmount)
	item.Note = ptrSQLString(row.Note)
	return item
}

// CreditActivityRowsToResponse maps merged ledger rows to the list API.
func CreditActivityRowsToResponse(rows []entity.CreditActivityRow, total, limit, offset int) dto.CreditActivityListResponse {
	out := make([]dto.CreditActivityItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, CreditActivityRowToItem(row))
	}
	return dto.CreditActivityListResponse{Items: out, Total: total, Limit: limit, Offset: offset}
}

// TopupResultToResponse maps domain top-up outcome to the API response (only fields we expose).
func TopupResultToResponse(r *entity.TopupResult) *dto.TopupResponse {
	if r == nil {
		return nil
	}
	return &dto.TopupResponse{
		ChargeID:     r.ChargeID,
		QRCodeURL:    r.QRCodeURL,
		Status:       r.Status,
		PaidAmount:   r.PaidAmount,
		FeeAmount:    r.FeeAmount,
		CreditAmount: r.CreditAmount,
	}
}

// TopupRequestToInput maps HTTP input plus authenticated user to a domain top-up input.
func TopupRequestToInput(userID string, req *dto.TopupRequest) entity.TopupInput {
	if req == nil {
		return entity.TopupInput{UserID: userID}
	}
	return entity.TopupInput{UserID: userID, Amount: req.Amount}
}

// WithdrawRequestToInput maps HTTP input plus authenticated user to a domain withdraw input.
func WithdrawRequestToInput(userID string, req *dto.WithdrawRequest) entity.WithdrawInput {
	if req == nil {
		return entity.WithdrawInput{UserID: userID}
	}
	return entity.WithdrawInput{UserID: userID, Amount: req.Amount}
}

// WithdrawResultToResponse maps domain withdraw outcome to the API response.
func WithdrawResultToResponse(r *entity.WithdrawResult) *dto.WithdrawResponse {
	if r == nil {
		return nil
	}
	return &dto.WithdrawResponse{
		WithdrawalID:      r.WithdrawalID,
		Amount:            r.Amount,
		FeeAmount:         r.FeeAmount,
		TransferAmount:    r.TransferAmount,
		Status:            r.Status,
		StatusLabel:       entity.WithdrawStatusLabelTH(r.Status),
		OmiseTransferID:   r.OmiseTransferID,
		BalanceAfter:      r.BalanceAfter,
		BankAccountName:   r.BankAccountName,
		BankAccountNumber: r.BankAccountNumber,
		BankCode:          r.BankCode,
	}
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
		key, _ := payload["key"].(string)
		if strings.HasPrefix(key, "transfer.") {
			return empty, false
		}
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

func transferFieldsFromData(data map[string]any) (id, status string) {
	if data == nil {
		return "", ""
	}
	if objType, _ := data["object"].(string); objType == "transfer" {
		id, _ = data["id"].(string)
		status, _ = data["status"].(string)
		return strings.TrimSpace(id), strings.TrimSpace(status)
	}
	if nested, ok := data["object"].(map[string]any); ok {
		if objType, _ := nested["object"].(string); objType == "transfer" {
			id, _ = nested["id"].(string)
			status, _ = nested["status"].(string)
		}
	}
	return strings.TrimSpace(id), strings.TrimSpace(status)
}

// WebhookPayloadToTransfer extracts transfer fields from an Omise event webhook.
func WebhookPayloadToTransfer(payload map[string]any) (entity.WebhookTransfer, bool) {
	var empty entity.WebhookTransfer
	if payload == nil {
		return empty, false
	}
	objectType, _ := payload["object"].(string)
	if objectType == "transfer" {
		id, status := transferFieldsFromData(payload)
		if id == "" {
			return empty, false
		}
		return entity.WebhookTransfer{TransferID: id, Status: status}, true
	}
	if objectType != "event" {
		return empty, false
	}
	key, _ := payload["key"].(string)
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, "transfer.") {
		return empty, false
	}
	dataMap, ok := payload["data"].(map[string]any)
	if !ok {
		return empty, false
	}
	id, status := transferFieldsFromData(dataMap)
	if id == "" {
		return empty, false
	}
	return entity.WebhookTransfer{TransferID: id, Status: status, EventKey: key}, true
}
