package service

import (
	"context"

	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
)

// FeeRates returns fee policy (DB business rules + env provider rates).
func (s *WalletService) FeeRates(ctx context.Context) dto.FeeRatesResponse {
	cfg := s.fees(ctx)
	return dto.FeeRatesResponse{
		MinTopupGrossTHB:           cfg.MinTopupGrossTHB,
		MinWithdrawCreditTHB:       cfg.MinWithdrawCreditTHB,
		TopupFeePercent:            cfg.TopupFeePercentDisplay(),
		TopupFeePPM:                cfg.OmisePromptPayFeePPM,
		WithdrawFeeTHB:             cfg.OmiseTransferFeeTHB,
		AuctionFeeNormalPct:        cfg.AuctionPlatformFeeNormalPct,
		AuctionFeeEarlyPct:         cfg.AuctionPlatformFeeEarlyPct,
		AuctionSellerKeepNormalPct: cfg.AuctionSellerKeepNormalPct,
		AuctionSellerKeepEarlyPct:  cfg.AuctionSellerKeepEarlyPct,
	}
}
