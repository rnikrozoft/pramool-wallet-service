package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rnikrozoft/pramool-wallet-service/internal/config"
	"github.com/rnikrozoft/pramool-wallet-service/internal/fees"
	"github.com/rnikrozoft/pramool-wallet-service/internal/money"
	"github.com/rnikrozoft/pramool-wallet-service/internal/omisehttp"
	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
	"github.com/rnikrozoft/pramool-wallet-service/repository"
)

type WalletService struct {
	repository     *repository.WalletRepository
	omiseSecretKey string
	httpClient     *http.Client
	feesLoader     *WalletFeesLoader
}

func NewWalletService(omiseSecretKey string, repository *repository.WalletRepository, httpClient *http.Client, feesLoader *WalletFeesLoader) *WalletService {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	return &WalletService{
		repository:     repository,
		omiseSecretKey: strings.TrimSpace(omiseSecretKey),
		httpClient:     httpClient,
		feesLoader:     feesLoader,
	}
}

func (s *WalletService) fees(ctx context.Context) config.WalletFeesConfig {
	return s.feesLoader.MustGet(ctx)
}

func (s *WalletService) feesCalc(ctx context.Context) *fees.Calculator {
	return fees.NewCalculator(s.fees(ctx))
}

func (s *WalletService) NewChargeID() string {
	return "chrg_local_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func (s *WalletService) CreatePromptPayTopup(ctx context.Context, in entity.TopupInput) (*entity.TopupResult, error) {
	if s.omiseSecretKey == "" {
		return nil, errors.New("missing omise secret key")
	}
	userID := strings.TrimSpace(in.UserID)
	gross := in.Amount
	if err := money.ValidatePositiveBaht(gross); err != nil {
		return nil, err
	}
	if gross < s.fees(ctx).MinTopupGrossTHB {
		return nil, fmt.Errorf("minimum top-up is %d baht", s.fees(ctx).MinTopupGrossTHB)
	}
	banned, err := s.repository.IsUserBanned(ctx, userID)
	if err != nil {
		return nil, err
	}
	if banned {
		return nil, repository.ErrTopupBanned
	}
	fee := s.feesCalc(ctx).TopupFee(gross)
	credit := s.feesCalc(ctx).TopupNetCredit(gross)
	if credit < 1 {
		return nil, errors.New("amount too small after payment fee")
	}

	if res, ok, err := s.tryResumePendingTopup(ctx, userID, gross, fee, credit); err != nil {
		return nil, err
	} else if ok {
		return res, nil
	}
	return s.createNewPromptPayTopup(ctx, userID, gross, fee, credit)
}

// TryResumePendingTopup returns an existing Omise charge QR when the user still has an unpaid top-up for the same amount.
func (s *WalletService) TryResumePendingTopup(ctx context.Context, userID string, gross int64) (*entity.TopupResult, bool, error) {
	if err := money.ValidatePositiveBaht(gross); err != nil {
		return nil, false, err
	}
	if gross < s.fees(ctx).MinTopupGrossTHB {
		return nil, false, fmt.Errorf("minimum top-up is %d baht", s.fees(ctx).MinTopupGrossTHB)
	}
	fee := s.feesCalc(ctx).TopupFee(gross)
	credit := s.feesCalc(ctx).TopupNetCredit(gross)
	return s.tryResumePendingTopup(ctx, strings.TrimSpace(userID), gross, fee, credit)
}

// SyncTopupChargeStatus loads charge state from Omise, updates the local row, and returns the current status.
func (s *WalletService) SyncTopupChargeStatus(ctx context.Context, userID, chargeID string) (*entity.TopupStatusResult, error) {
	userID = strings.TrimSpace(userID)
	chargeID = strings.TrimSpace(chargeID)
	if chargeID == "" {
		return nil, errors.New("empty charge id")
	}
	row, err := s.repository.GetTopupTransaction(userID, chargeID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.New("ไม่พบรายการเติมเงิน")
	}

	details, err := s.fetchChargeDetails(ctx, chargeID)
	if err != nil {
		return nil, err
	}

	if details.Paid && strings.EqualFold(details.Status, "successful") {
		if err := s.ProcessWebhookCharge(chargeID, details.Status, details.Paid); err != nil {
			return nil, err
		}
	} else {
		if err := s.repository.UpdateTransactionStatus(chargeID, details.Status, details.Paid); err != nil {
			return nil, err
		}
	}

	updated, err := s.repository.GetTopupTransaction(userID, chargeID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errors.New("ไม่พบรายการเติมเงิน")
	}

	if details.DisputeID != "" || details.DisputeStatus != "" {
		if err := s.applyChargeDisputeFromDetails(chargeID, details); err != nil {
			return nil, err
		}
		updated, err = s.repository.GetTopupTransaction(userID, chargeID)
		if err != nil {
			return nil, err
		}
		if updated == nil {
			return nil, errors.New("ไม่พบรายการเติมเงิน")
		}
		details.DisputeID = updated.DisputeID
		details.DisputeStatus = updated.DisputeStatus
	}

	return topupStatusFromRow(updated, details), nil
}

func topupStatusFromRow(row *entity.Transaction, details omiseChargeDetails) *entity.TopupStatusResult {
	feeAmt := row.FeeAmount
	creditAmt := row.CreditAmount
	if feeAmt <= 0 {
		feeAmt = 0
	}
	if creditAmt <= 0 {
		creditAmt = row.Amount
	}
	qrURL := strings.TrimSpace(row.QRCodeURL)
	if qrURL == "" {
		qrURL = strings.TrimSpace(details.QRURL)
	}
	expired := details.Expired || strings.EqualFold(strings.TrimSpace(details.Status), "expired")
	disputeStatus := strings.TrimSpace(row.DisputeStatus)
	if disputeStatus == "" {
		disputeStatus = strings.TrimSpace(details.DisputeStatus)
	}
	status := strings.TrimSpace(row.Status)
	if status == "" {
		status = strings.TrimSpace(details.Status)
	}
	return &entity.TopupStatusResult{
		ChargeID:      row.ChargeID,
		QRCodeURL:     qrURL,
		Status:        status,
		Paid:          details.Paid || row.Paid,
		Credited:      row.Credited,
		Expired:       expired,
		ExpiresAt:     details.ExpiresAt,
		DisputeStatus: disputeStatus,
		PaidAmount:    row.Amount,
		FeeAmount:     feeAmt,
		CreditAmount:  creditAmt,
	}
}

func (s *WalletService) tryResumePendingTopup(ctx context.Context, userID string, gross, fee, credit int64) (*entity.TopupResult, bool, error) {
	row, err := s.repository.FindLatestPendingTopup(userID, gross)
	if err != nil || row == nil {
		return nil, false, err
	}

	details, err := s.fetchChargeDetails(ctx, row.ChargeID)
	if err != nil {
		return nil, false, nil
	}

	_ = s.repository.UpdateTransactionStatus(row.ChargeID, details.Status, details.Paid)

	if details.Paid && details.Status == "successful" {
		_ = s.ProcessWebhookCharge(row.ChargeID, details.Status, details.Paid)
		return &entity.TopupResult{
			ChargeID:     row.ChargeID,
			QRCodeURL:    row.QRCodeURL,
			Status:       details.Status,
			PaidAmount:   row.Amount,
			FeeAmount:    row.FeeAmount,
			CreditAmount: row.CreditAmount,
			ExpiresAt:    details.ExpiresAt,
			Resumed:      true,
		}, true, nil
	}

	if topupChargeTerminal(details.Status, details.Paid) {
		return nil, false, nil
	}

	qrURL := strings.TrimSpace(details.QRURL)
	if qrURL == "" {
		qrURL = strings.TrimSpace(row.QRCodeURL)
	}
	if qrURL == "" && details.SourceID != "" {
		qrURL, _ = s.getPromptPayQRFromSource(ctx, details.SourceID)
	}
	if qrURL == "" {
		return nil, false, nil
	}
	if qrURL != row.QRCodeURL {
		_ = s.repository.UpdateTransactionQRCode(row.ChargeID, qrURL)
	}

	feeAmt := row.FeeAmount
	if feeAmt <= 0 {
		feeAmt = fee
	}
	creditAmt := row.CreditAmount
	if creditAmt <= 0 {
		creditAmt = credit
	}

	return &entity.TopupResult{
		ChargeID:     row.ChargeID,
		QRCodeURL:    qrURL,
		Status:       details.Status,
		PaidAmount:   row.Amount,
		FeeAmount:    feeAmt,
		CreditAmount: creditAmt,
		ExpiresAt:    details.ExpiresAt,
		Resumed:      true,
	}, true, nil
}

func topupChargeTerminal(status string, paid bool) bool {
	if paid && strings.EqualFold(strings.TrimSpace(status), "successful") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "expired", "reversed":
		return true
	default:
		return false
	}
}

func (s *WalletService) createNewPromptPayTopup(ctx context.Context, userID string, gross, fee, credit int64) (*entity.TopupResult, error) {
	sourceID, qrURL, err := s.createPromptPaySource(ctx, gross)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("amount", fmt.Sprintf("%d", gross*100))
	values.Set("currency", "thb")
	values.Set("source", sourceID)
	values.Set("description", fmt.Sprintf("Topup credits for user %s", userID))

	req, err := http.NewRequest(http.MethodPost, "https://api.omise.co/charges", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise charge failed")
	if err != nil {
		return nil, err
	}

	var charge omiseChargeJSON
	if err := json.Unmarshal(body, &charge); err != nil {
		return nil, err
	}
	if charge.ID == "" {
		return nil, errors.New("cannot get charge id from omise")
	}

	if qrURL == "" {
		qrURL = charge.Source.ScannableCode.Image.DownloadURI
	}
	if qrURL == "" {
		qrURL, _ = s.getPromptPayQRFromSource(ctx, sourceID)
	}
	if qrURL == "" {
		return nil, errors.New("cannot get promptpay qr data from source/charge")
	}

	if err := s.repository.InsertTransaction(charge.ID, userID, gross, fee, credit, "pending", false, false, qrURL); err != nil {
		return nil, err
	}
	if err := s.repository.UpdateTransactionStatus(charge.ID, charge.Status, charge.Paid); err != nil {
		return nil, err
	}

	return &entity.TopupResult{
		ChargeID:     charge.ID,
		QRCodeURL:    qrURL,
		Status:       charge.Status,
		PaidAmount:   gross,
		FeeAmount:    fee,
		CreditAmount: credit,
		ExpiresAt:    charge.ExpiresAt,
	}, nil
}

func (s *WalletService) createPromptPaySource(ctx context.Context, amount int64) (string, string, error) {
	values := url.Values{}
	values.Set("type", "promptpay")
	values.Set("amount", fmt.Sprintf("%d", amount*100))
	values.Set("currency", "thb")

	req, err := http.NewRequest(http.MethodPost, "https://api.omise.co/sources", strings.NewReader(values.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise source failed")
	if err != nil {
		return "", "", err
	}

	var source struct {
		ID            string `json:"id"`
		ScannableCode struct {
			Image struct {
				DownloadURI string `json:"download_uri"`
			} `json:"image"`
		} `json:"scannable_code"`
	}
	if err := json.Unmarshal(body, &source); err != nil {
		return "", "", err
	}
	if source.ID == "" {
		return "", "", errors.New("cannot get promptpay source id")
	}
	return source.ID, source.ScannableCode.Image.DownloadURI, nil
}

func (s *WalletService) getPromptPayQRFromSource(ctx context.Context, sourceID string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.omise.co/sources/%s", sourceID), nil)
	if err != nil {
		return "", err
	}

	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise source status failed")
	if err != nil {
		return "", err
	}

	var source struct {
		ScannableCode struct {
			Image struct {
				DownloadURI string `json:"download_uri"`
			} `json:"image"`
		} `json:"scannable_code"`
	}
	if err := json.Unmarshal(body, &source); err != nil {
		return "", err
	}
	return source.ScannableCode.Image.DownloadURI, nil
}

// FetchChargeStateFromAPI loads charge paid/status from Omise (GET /charges/:id).
func (s *WalletService) FetchChargeStateFromAPI(ctx context.Context, chargeID string) (status string, paid bool, err error) {
	details, err := s.fetchChargeDetails(ctx, chargeID)
	if err != nil {
		return "", false, err
	}
	return details.Status, details.Paid, nil
}

type omiseChargeDetails struct {
	Status        string
	Paid          bool
	Expired       bool
	ExpiresAt     string
	DisputeID     string
	DisputeStatus string
	QRURL         string
	SourceID      string
}

type omiseChargeJSON struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Paid      bool   `json:"paid"`
	Expired   bool   `json:"expired"`
	ExpiresAt string `json:"expires_at"`
	Dispute   *struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"dispute"`
	Source struct {
		ID            string `json:"id"`
		ScannableCode struct {
			Image struct {
				DownloadURI string `json:"download_uri"`
			} `json:"image"`
		} `json:"scannable_code"`
	} `json:"source"`
}

func omiseChargeDetailsFromJSON(ch omiseChargeJSON) omiseChargeDetails {
	d := omiseChargeDetails{
		Status:    strings.TrimSpace(ch.Status),
		Paid:      ch.Paid,
		Expired:   ch.Expired,
		ExpiresAt: strings.TrimSpace(ch.ExpiresAt),
		QRURL:     strings.TrimSpace(ch.Source.ScannableCode.Image.DownloadURI),
		SourceID:  strings.TrimSpace(ch.Source.ID),
	}
	if ch.Dispute != nil {
		d.DisputeID = strings.TrimSpace(ch.Dispute.ID)
		d.DisputeStatus = strings.TrimSpace(ch.Dispute.Status)
	}
	return d
}

func (s *WalletService) fetchChargeDetails(ctx context.Context, chargeID string) (omiseChargeDetails, error) {
	var empty omiseChargeDetails
	chargeID = strings.TrimSpace(chargeID)
	if chargeID == "" {
		return empty, errors.New("empty charge id")
	}
	if s.omiseSecretKey == "" {
		return empty, errors.New("missing omise secret key")
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.omise.co/charges/%s", url.PathEscape(chargeID)), nil)
	if err != nil {
		return empty, err
	}
	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise charge fetch failed")
	if err != nil {
		return empty, err
	}
	var ch omiseChargeJSON
	if err := json.Unmarshal(body, &ch); err != nil {
		return empty, err
	}
	return omiseChargeDetailsFromJSON(ch), nil
}

func (s *WalletService) ProcessWebhookCharge(chargeID, status string, paid bool) error {
	if err := s.repository.UpdateTransactionStatus(chargeID, status, paid); err != nil {
		return err
	}
	if !paid || status != "successful" {
		return nil
	}

	rowsUpdated, err := s.repository.UpdateTransactionSetCredited(chargeID)
	if err != nil {
		return err
	}
	if rowsUpdated == 0 {
		return nil
	}

	topup, err := s.repository.GetTransactionCreditFields(chargeID)
	if err != nil {
		return err
	}

	credit := topup.CreditAmount
	if credit <= 0 {
		credit = s.feesCalc(context.Background()).TopupNetCredit(topup.Amount)
	}
	return s.repository.AddUserCredit(topup.UserID, credit)
}

func (s *WalletService) ListCreditActivity(userID string, limit, offset int, filter, sortKey, sortOrder string) ([]entity.CreditActivityRow, int, error) {
	total, err := s.repository.CountCreditActivity(userID, filter)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.repository.ListCreditActivity(userID, limit, offset, filter, sortKey, sortOrder)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// decodeOmiseWebhookSecret decodes the webhook signing key from the Omise Dashboard (base64).
// Accepts standard / URL-safe / raw variants—mis-pasted secrets are a common cause of 401 on webhooks.
func decodeOmiseWebhookSecret(secretBase64 string) ([]byte, error) {
	s := strings.Trim(strings.TrimSpace(secretBase64), "\"'")
	if s == "" {
		return nil, fmt.Errorf("empty webhook secret")
	}
	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, dec := range decoders {
		b, err := dec.DecodeString(s)
		if err == nil && len(b) > 0 {
			return b, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("invalid webhook secret encoding")
}

// VerifyOmiseSignature matches Omise docs: HMAC-SHA256 over "timestamp.rawBody" with base64-decoded secret;
// Omise-Signature header is comma-separated hex digests (secret rotation). Timestamp is required when signing is used.
func (s *WalletService) VerifyOmiseSignature(secretBase64, signatureHeader, timestamp string, body []byte) bool {
	secret, err := decodeOmiseWebhookSecret(secretBase64)
	if err != nil || signatureHeader == "" {
		return false
	}
	ts := strings.TrimSpace(timestamp)
	if ts == "" {
		return false
	}
	// Build timestamp + "." + body without string(body) allocation pitfalls on binary-safe payload.
	signedLen := len(ts) + 1 + len(body)
	signedPayload := make([]byte, 0, signedLen)
	signedPayload = append(signedPayload, ts...)
	signedPayload = append(signedPayload, '.')
	signedPayload = append(signedPayload, body...)

	mac := hmac.New(sha256.New, secret)
	mac.Write(signedPayload)
	expected := mac.Sum(nil)

	for _, part := range strings.Split(signatureHeader, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(part), "sha256="), "v1=")
		part = strings.Trim(part, "\"'")
		if part == "" {
			continue
		}
		got, err := hex.DecodeString(part)
		if err != nil || len(got) != len(expected) {
			continue
		}
		if hmac.Equal(got, expected) {
			return true
		}
	}
	return false
}

