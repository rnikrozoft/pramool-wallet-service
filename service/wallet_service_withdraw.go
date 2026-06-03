package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rnikrozoft/pramool-wallet-service/internal/money"
	"github.com/rnikrozoft/pramool-wallet-service/internal/omisehttp"
	"github.com/rnikrozoft/pramool-wallet-service/model/entity"
	"github.com/rnikrozoft/pramool-wallet-service/repository"
)

// WithdrawCredit sends user credit to their registered bank account via Omise Transfer.
func (s *WalletService) WithdrawCredit(ctx context.Context, in entity.WithdrawInput) (*entity.WithdrawResult, error) {
	if s.omiseSecretKey == "" {
		return nil, errors.New("missing omise secret key")
	}
	userID := strings.TrimSpace(in.UserID)
	amount := in.Amount
	if err := money.ValidatePositiveBaht(amount); err != nil {
		return nil, err
	}
	banned, err := s.repository.IsUserBanned(ctx, userID)
	if err != nil {
		return nil, err
	}
	if banned {
		return nil, repository.ErrWithdrawBanned
	}
	if amount < s.fees(ctx).MinWithdrawCreditTHB {
		return nil, fmt.Errorf("minimum withdrawal is %d baht", s.fees(ctx).MinWithdrawCreditTHB)
	}
	fee := s.fees(ctx).OmiseTransferFeeTHB
	transferAmount := s.feesCalc(ctx).WithdrawNetTransfer(amount)
	if transferAmount < 1 {
		return nil, fmt.Errorf("amount must exceed transfer fee (%d baht)", fee)
	}

	profile, err := s.repository.GetUserPayoutProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := repository.ValidatePayoutProfile(profile); err != nil {
		return nil, err
	}
	if profile.Credit < 0 {
		return nil, repository.ErrCreditDebt
	}
	if profile.Credit < amount {
		return nil, repository.ErrInsufficientCredit
	}

	sellerN, err := s.repository.CountUserFulfillmentBlocks(ctx, userID)
	if err != nil {
		return nil, err
	}
	if sellerN > 0 {
		return nil, fmt.Errorf("%w: %s", repository.ErrWithdrawalBlocked, repository.WithdrawalBlockedReason(sellerN))
	}

	brand, ok := omiseBankBrand(profile.BankCode)
	if !ok {
		return nil, fmt.Errorf("unsupported bank for payout: %s", profile.BankCode)
	}

	withdrawalID, _, balanceAfter, err := s.repository.ReserveWithdrawal(ctx, userID, amount, fee, transferAmount)
	if err != nil {
		return nil, err
	}

	recipientID := strings.TrimSpace(profile.OmiseRecipientID)
	if recipientID == "" {
		recipientID, err = s.createOmiseRecipient(ctx, profile, brand)
		if err != nil {
			_ = s.repository.FailWithdrawal(ctx, withdrawalID, err.Error())
			return nil, err
		}
		if err := s.repository.SetUserOmiseRecipientID(ctx, userID, recipientID); err != nil {
			_ = s.repository.FailWithdrawal(ctx, withdrawalID, err.Error())
			return nil, err
		}
	}

	transferID, transferStatus, err := s.createOmiseTransfer(ctx, recipientID, transferAmount)
	if err != nil {
		_ = s.repository.FailWithdrawal(ctx, withdrawalID, err.Error())
		return nil, err
	}

	finalStatus := entity.WithdrawStatusFromOmiseTransfer(transferStatus)
	if finalStatus == entity.WithdrawStatusFailed {
		_ = s.repository.FailWithdrawal(ctx, withdrawalID, "omise transfer failed")
		return nil, errors.New("omise transfer failed")
	}

	if err := s.repository.CompleteWithdrawal(ctx, withdrawalID, finalStatus, transferID, recipientID); err != nil {
		return nil, err
	}

	masked := maskAccountNumber(profile.BankAccountNumber)
	return &entity.WithdrawResult{
		WithdrawalID:      withdrawalID,
		Amount:            amount,
		FeeAmount:         fee,
		TransferAmount:    transferAmount,
		Status:            finalStatus,
		OmiseTransferID:   transferID,
		BalanceAfter:      balanceAfter,
		BankAccountName:   profile.BankAccountName,
		BankAccountNumber: masked,
		BankCode:          profile.BankCode,
	}, nil
}

func maskAccountNumber(num string) string {
	num = strings.TrimSpace(num)
	if len(num) <= 4 {
		return num
	}
	return strings.Repeat("*", len(num)-4) + num[len(num)-4:]
}

func (s *WalletService) createOmiseRecipient(ctx context.Context, p *entity.UserPayoutProfile, brand string) (string, error) {
	values := url.Values{}
	values.Set("name", strings.TrimSpace(p.BankAccountName))
	values.Set("type", "individual")
	values.Set("bank_account[brand]", brand)
	values.Set("bank_account[number]", strings.TrimSpace(p.BankAccountNumber))
	values.Set("bank_account[name]", strings.TrimSpace(p.BankAccountName))

	req, err := http.NewRequest(http.MethodPost, "https://api.omise.co/recipients", strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise recipient failed")
	if err != nil {
		return "", err
	}
	var out struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("cannot get recipient id from omise")
	}
	return out.ID, nil
}

func (s *WalletService) createOmiseTransfer(ctx context.Context, recipientID string, amountTHB int64) (transferID, status string, err error) {
	values := url.Values{}
	values.Set("amount", fmt.Sprintf("%d", amountTHB*100))
	values.Set("recipient", recipientID)

	req, err := http.NewRequest(http.MethodPost, "https://api.omise.co/transfers", strings.NewReader(values.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := omisehttp.Do(ctx, s.httpClient, s.omiseSecretKey, req, "omise transfer failed")
	if err != nil {
		return "", "", err
	}
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", "", err
	}
	if out.ID == "" {
		return "", "", errors.New("cannot get transfer id from omise")
	}
	return out.ID, strings.TrimSpace(out.Status), nil
}
