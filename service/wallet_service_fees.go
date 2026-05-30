package service

import (
	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
)

// FeeRates returns fee policy from config (also exposed at GET /wallet/fees for frontend).
func (s *WalletService) FeeRates() dto.FeeRatesResponse {
	cfg := s.feesCfg
	return dto.FeeRatesResponse{
		MinTopupGrossTHB:            cfg.MinTopupGrossTHB,
		MinWithdrawCreditTHB:        cfg.MinWithdrawCreditTHB,
		TopupFeePercent:             cfg.TopupFeePercentDisplay(),
		TopupFeePPM:                 cfg.OmisePromptPayFeePPM,
		WithdrawFeeTHB:              cfg.OmiseTransferFeeTHB,
		AuctionFeeNormalPct:         cfg.AuctionPlatformFeeNormalPct,
		AuctionFeeEarlyPct:          cfg.AuctionPlatformFeeEarlyPct,
		AuctionSellerKeepNormalPct:  cfg.AuctionSellerKeepNormalPct,
		AuctionSellerKeepEarlyPct:   cfg.AuctionSellerKeepEarlyPct,
	}
}
