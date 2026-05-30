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
	feesCfg        config.WalletFeesConfig
	feesCalc       *fees.Calculator
}

func NewWalletService(omiseSecretKey string, repository *repository.WalletRepository, httpClient *http.Client, feesCfg config.WalletFeesConfig) *WalletService {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	return &WalletService{
		repository:     repository,
		omiseSecretKey: strings.TrimSpace(omiseSecretKey),
		httpClient:     httpClient,
		feesCfg:        feesCfg,
		feesCalc:       fees.NewCalculator(feesCfg),
	}
}

func (s *WalletService) NewChargeID() string {
	return "chrg_local_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func (s *WalletService) CreatePromptPayTopup(ctx context.Context, in entity.TopupInput) (*entity.TopupResult, error) {
	if s.omiseSecretKey == "" {
		return nil, errors.New("missing omise secret key")
	}
	userID := in.UserID
	gross := in.Amount
	if err := money.ValidatePositiveBaht(gross); err != nil {
		return nil, err
	}
	if gross < s.feesCfg.MinTopupGrossTHB {
		return nil, fmt.Errorf("minimum top-up is %d baht", s.feesCfg.MinTopupGrossTHB)
	}
	fee := s.feesCalc.TopupFee(gross)
	credit := s.feesCalc.TopupNetCredit(gross)
	if credit < 1 {
		return nil, errors.New("amount too small after payment fee")
	}

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

	var charge struct {
		ID     string `json:"id"`
		Paid   bool   `json:"paid"`
		Status string `json:"status"`
		Source struct {
			ScannableCode struct {
				Image struct {
					DownloadURI string `json:"download_uri"`
				} `json:"image"`
			} `json:"scannable_code"`
		} `json:"source"`
	}
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

	if err := s.repository.InsertTransaction(charge.ID, userID, gross, fee, credit, "pending", false, false); err != nil {
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
// Use when webhook requests have no Omise-Signature headers — Omise only sends signatures after you configure a webhook secret in the dashboard; otherwise use this event-verification path per https://docs.omise.co/api-webhooks#protecting-your-endpoints
func (s *WalletService) FetchChargeStateFromAPI(ctx context.Context, chargeID string) (status string, paid bool, err error) {
	chargeID = strings.TrimSpace(chargeID)
	if chargeID == "" {
		return "", false, errors.New("empty charge id")
	}
	if s.omiseSecretKey == "" {
		return "", false, errors.New("missing omise secret key")
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.omise.co/charges/%s", url.PathEscape(chargeID)), nil)
	if err != nil {
		return "", false, err
	}
	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise charge fetch failed")
	if err != nil {
		return "", false, err
	}
	var ch struct {
		Status string `json:"status"`
		Paid   bool   `json:"paid"`
	}
	if err := json.Unmarshal(body, &ch); err != nil {
		return "", false, err
	}
	return strings.TrimSpace(ch.Status), ch.Paid, nil
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
		credit = s.feesCalc.TopupNetCredit(topup.Amount)
	}
	return s.repository.AddUserCredit(topup.UserID, credit)
}

func (s *WalletService) ListCreditActivity(userID string, limit, offset int, filter string) ([]entity.CreditActivityRow, int, error) {
	total, err := s.repository.CountCreditActivity(userID, filter)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.repository.ListCreditActivity(userID, limit, offset, filter)
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

