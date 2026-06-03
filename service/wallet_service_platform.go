package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
)

func platformAvailableBalance(total, withdrawn int64) int64 {
	available := total - withdrawn
	if available < 0 {
		return 0
	}
	return available
}

func (s *WalletService) PlatformRevenue(ctx context.Context) (*dto.PlatformRevenueResponse, error) {
	rev, err := s.repository.SummarizePlatformFees(ctx)
	if err != nil {
		return nil, err
	}
	withdrawn, err := s.repository.SumPlatformWithdrawn(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.PlatformRevenueResponse{
		TotalPlatformShareBaht:  rev.TotalPlatformShareBaht,
		CompletedPayoutAuctions: rev.CompletedPayoutAuctions,
		TotalWithdrawnBaht:      withdrawn,
		AvailableBalanceBaht:    platformAvailableBalance(rev.TotalPlatformShareBaht, withdrawn),
	}, nil
}

func (s *WalletService) PlatformWithdraw(ctx context.Context, adminID int, amountBaht int64, platformRecipient string) (*dto.PlatformWithdrawResponse, error) {
	if amountBaht < 1 {
		return nil, errors.New("amount must be at least 1 baht")
	}
	if strings.TrimSpace(platformRecipient) == "" {
		return nil, errors.New("PLATFORM_OMISE_RECIPIENT_ID not configured")
	}
	if s.omiseSecretKey == "" {
		return nil, errors.New("missing omise secret key")
	}

	rev, err := s.PlatformRevenue(ctx)
	if err != nil {
		return nil, err
	}
	if amountBaht > rev.AvailableBalanceBaht {
		return nil, errors.New("insufficient platform balance")
	}

	withdrawalID, err := s.repository.InsertPlatformWithdrawal(ctx, adminID, amountBaht)
	if err != nil {
		return nil, err
	}

	transferID, omiseStatus, err := s.createOmiseTransfer(ctx, platformRecipient, amountBaht)
	if err != nil {
		_ = s.repository.CompletePlatformWithdrawal(ctx, withdrawalID, "failed", "", "", err.Error())
		return nil, err
	}

	status := "completed"
	if omiseStatus == "failed" {
		status = "failed"
	}
	if err := s.repository.CompletePlatformWithdrawal(ctx, withdrawalID, status, transferID, omiseStatus, ""); err != nil {
		return nil, err
	}
	if status == "failed" {
		return nil, errors.New("omise transfer failed")
	}

	updated, err := s.PlatformRevenue(ctx)
	if err != nil {
		return nil, err
	}

	return &dto.PlatformWithdrawResponse{
		WithdrawalID:           withdrawalID,
		AmountBaht:             amountBaht,
		Status:                 status,
		OmiseTransferID:        transferID,
		OmiseStatus:            omiseStatus,
		TotalPlatformShareBaht: updated.TotalPlatformShareBaht,
		TotalWithdrawnBaht:     updated.TotalWithdrawnBaht,
		AvailableBalanceBaht:   updated.AvailableBalanceBaht,
	}, nil
}

func (s *WalletService) ListPlatformWithdrawals(ctx context.Context, limit int) (*dto.PlatformWithdrawListResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.repository.ListPlatformWithdrawals(ctx, limit)
	if err != nil {
		return nil, err
	}
	items := make([]dto.PlatformWithdrawListItem, 0, len(rows))
	for _, r := range rows {
		item := dto.PlatformWithdrawListItem{
			WithdrawalID: r.WithdrawalID,
			AmountBaht:   r.AmountBaht,
			Status:       r.Status,
			CreatedAt:    r.CreatedAt.Format(time.RFC3339),
		}
		if r.OmiseTransferID.Valid {
			item.OmiseTransferID = r.OmiseTransferID.String
		}
		if r.OmiseStatus.Valid {
			item.OmiseStatus = r.OmiseStatus.String
		}
		items = append(items, item)
	}
	return &dto.PlatformWithdrawListResponse{Items: items}, nil
}
