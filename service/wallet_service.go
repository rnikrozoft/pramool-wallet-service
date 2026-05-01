package service

import (
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

	"github.com/rnikrozoft/pramool-wallet-service/internal/omisehttp"
	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
	"github.com/rnikrozoft/pramool-wallet-service/repository"
)

type WalletService struct {
	repository     *repository.WalletRepository
	omiseSecretKey string
}

func NewWalletService(omiseSecretKey string, repository *repository.WalletRepository) *WalletService {
	return &WalletService{
		repository:     repository,
		omiseSecretKey: strings.TrimSpace(omiseSecretKey),
	}
}

const transactionListLimit = 100

func (s *WalletService) NewChargeID() string {
	return "chrg_local_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func (s *WalletService) CreatePromptPayTopup(in entity.TopupInput) (*entity.TopupResult, error) {
	if s.omiseSecretKey == "" {
		return nil, errors.New("missing omise secret key")
	}
	userID := in.UserID
	amount := in.Amount

	sourceID, qrURL, err := s.createPromptPaySource(amount)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("amount", fmt.Sprintf("%d", amount*100))
	values.Set("currency", "thb")
	values.Set("source", sourceID)
	values.Set("description", fmt.Sprintf("Topup credits for user %s", userID))

	req, err := http.NewRequest(http.MethodPost, "https://api.omise.co/charges", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := omisehttp.Do(http.DefaultClient, s.omiseSecretKey, req, "omise charge failed")
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
		qrURL, _ = s.getPromptPayQRFromSource(sourceID)
	}
	if qrURL == "" {
		return nil, errors.New("cannot get promptpay qr data from source/charge")
	}

	if err := s.repository.InsertTransaction(charge.ID, userID, amount, "pending", false, false); err != nil {
		return nil, err
	}
	if err := s.repository.UpdateTransactionStatus(charge.ID, charge.Status, charge.Paid); err != nil {
		return nil, err
	}

	return &entity.TopupResult{
		ChargeID:  charge.ID,
		QRCodeURL: qrURL,
		Status:    charge.Status,
	}, nil
}

func (s *WalletService) createPromptPaySource(amount int64) (string, string, error) {
	values := url.Values{}
	values.Set("type", "promptpay")
	values.Set("amount", fmt.Sprintf("%d", amount*100))
	values.Set("currency", "thb")

	req, err := http.NewRequest(http.MethodPost, "https://api.omise.co/sources", strings.NewReader(values.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := omisehttp.Do(http.DefaultClient, s.omiseSecretKey, req, "omise source failed")
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

func (s *WalletService) getPromptPayQRFromSource(sourceID string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.omise.co/sources/%s", sourceID), nil)
	if err != nil {
		return "", err
	}

	body, err := omisehttp.Do(http.DefaultClient, s.omiseSecretKey, req, "omise source status failed")
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

	return s.repository.AddUserCredit(topup.UserID, topup.Amount)
}

func (s *WalletService) ListTransactions(userID string) ([]entity.Transaction, error) {
	return s.repository.ListTransactionsByUser(userID, transactionListLimit)
}

func (s *WalletService) VerifyOmiseSignature(secretBase64, signatureHeader, timestamp string, body []byte) bool {
	secret, err := base64.StdEncoding.DecodeString(strings.TrimSpace(secretBase64))
	if err != nil || signatureHeader == "" {
		return false
	}
	signedPayload := body
	if timestamp != "" {
		signedPayload = []byte(timestamp + "." + string(body))
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(signedPayload)
	expected := mac.Sum(nil)
	for _, part := range strings.Split(signatureHeader, ",") {
		sig := strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(part), "sha256=")), "\"'")
		if sig == "" {
			continue
		}
		got, err := hex.DecodeString(sig)
		if err == nil && hmac.Equal(got, expected) {
			return true
		}
	}
	return false
}
