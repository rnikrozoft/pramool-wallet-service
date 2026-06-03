package config

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseWalletFees(m map[string]any) (WalletFeesConfig, error) {
	if m == nil {
		return WalletFeesConfig{}, fmt.Errorf("platform_settings.fees missing")
	}
	minTopup, err := requireWalletInt64(m, "min_topup_gross_thb", 1)
	if err != nil {
		return WalletFeesConfig{}, err
	}
	minWithdraw, err := requireWalletInt64(m, "min_withdraw_credit_thb", 1)
	if err != nil {
		return WalletFeesConfig{}, err
	}
	ppm, err := requireWalletInt64(m, "omise_promptpay_fee_ppm", 1)
	if err != nil {
		return WalletFeesConfig{}, err
	}
	transferFee, err := requireWalletInt64(m, "omise_transfer_fee_thb", 0)
	if err != nil {
		return WalletFeesConfig{}, err
	}
	keepNormal, err := requireWalletInt64(m, "seller_keep_normal_pct", 1)
	if err != nil {
		return WalletFeesConfig{}, err
	}
	keepEarly, err := requireWalletInt64(m, "seller_keep_early_pct", 1)
	if err != nil {
		return WalletFeesConfig{}, err
	}
	if keepNormal > 100 || keepEarly > 100 {
		return WalletFeesConfig{}, fmt.Errorf("platform_settings.fees seller_keep_* must be <= 100")
	}
	return WalletFeesConfig{
		MinTopupGrossTHB:            minTopup,
		MinWithdrawCreditTHB:        minWithdraw,
		OmisePromptPayFeePPM:        ppm,
		OmiseTransferFeeTHB:         transferFee,
		AuctionSellerKeepNormalPct:  keepNormal,
		AuctionPlatformFeeNormalPct: 100 - keepNormal,
		AuctionSellerKeepEarlyPct:   keepEarly,
		AuctionPlatformFeeEarlyPct:  100 - keepEarly,
	}, nil
}

func requireWalletInt64(m map[string]any, key string, min int64) (int64, error) {
	raw, ok := m[key]
	if !ok || raw == nil {
		return 0, fmt.Errorf("platform_settings.fees.%s missing", key)
	}
	v := walletJSONInt64(m, key)
	if v < min {
		return 0, fmt.Errorf("platform_settings.fees.%s invalid: %d (min %d)", key, v, min)
	}
	return v, nil
}

func walletJSONInt64(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0
		}
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}
