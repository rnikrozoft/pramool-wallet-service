package fees

import (
	"testing"

	"github.com/rnikrozoft/pramool-wallet-service/internal/config"
)

func testWalletFeesConfig() config.WalletFeesConfig {
	return config.WalletFeesConfig{
		MinTopupGrossTHB:            100,
		MinWithdrawCreditTHB:        100,
		OmisePromptPayFeePPM:        17655,
		OmiseTransferFeeTHB:         21,
		AuctionPlatformFeeNormalPct: 25,
		AuctionPlatformFeeEarlyPct:  30,
		AuctionSellerKeepNormalPct:  75,
		AuctionSellerKeepEarlyPct:   70,
	}
}

func TestTopupNetCredit(t *testing.T) {
	calc := NewCalculator(testWalletFeesConfig())
	tests := []struct {
		gross  int64
		wantCr int64
	}{
		{100, 98},
		{1000, 982},
		{10000, 9823},
	}
	for _, tc := range tests {
		got := calc.TopupNetCredit(tc.gross)
		if got != tc.wantCr {
			t.Errorf("TopupNetCredit(%d) = %d, want %d (fee=%d)", tc.gross, got, tc.wantCr, calc.TopupFee(tc.gross))
		}
	}
}

func TestWithdrawNetTransfer(t *testing.T) {
	calc := NewCalculator(testWalletFeesConfig())
	if got := calc.WithdrawNetTransfer(100); got != 79 {
		t.Fatalf("WithdrawNetTransfer(100) = %d, want 79", got)
	}
	if got := calc.WithdrawNetTransfer(21); got != 0 {
		t.Fatalf("WithdrawNetTransfer(21) = %d, want 0", got)
	}
}

func TestParseWalletFeesMissing(t *testing.T) {
	_, err := config.ParseWalletFees(map[string]any{})
	if err == nil {
		t.Fatal("expected error for empty fees map")
	}
}
