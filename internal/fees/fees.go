package fees

import (
	"github.com/rnikrozoft/pramool-wallet-service/internal/config"
)

// Calculator applies WalletFeesConfig to top-up / withdraw amounts.
type Calculator struct {
	cfg config.WalletFeesConfig
}

func NewCalculator(cfg config.WalletFeesConfig) *Calculator {
	return &Calculator{cfg: cfg}
}

func (c *Calculator) Config() config.WalletFeesConfig {
	return c.cfg
}

// TopupFee estimates Omise PromptPay fee from gross THB paid (integer baht, rounded up).
func (c *Calculator) TopupFee(grossTHB int64) int64 {
	if grossTHB <= 0 || c.cfg.OmisePromptPayFeePPM <= 0 {
		return 0
	}
	return (grossTHB*c.cfg.OmisePromptPayFeePPM + 999_999) / 1_000_000
}

// TopupNetCredit is credit added after estimated PromptPay fee.
func (c *Calculator) TopupNetCredit(grossTHB int64) int64 {
	fee := c.TopupFee(grossTHB)
	if grossTHB <= fee {
		return 0
	}
	return grossTHB - fee
}

// WithdrawNetTransfer is approximate THB sent to the user's bank after Omise transfer fee.
func (c *Calculator) WithdrawNetTransfer(creditDeducted int64) int64 {
	fee := c.cfg.OmiseTransferFeeTHB
	if creditDeducted <= fee {
		return 0
	}
	return creditDeducted - fee
}
