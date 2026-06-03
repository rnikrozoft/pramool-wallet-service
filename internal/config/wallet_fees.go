package config

// WalletFeesConfig ค่านโยบายค่าธรรมเนียมที่แสดงใน UI และใช้คำนวณเครดิตสุทธิ
type WalletFeesConfig struct {
	MinTopupGrossTHB            int64
	MinWithdrawCreditTHB        int64
	OmisePromptPayFeePPM        int64
	OmiseTransferFeeTHB         int64
	AuctionPlatformFeeNormalPct int64
	AuctionPlatformFeeEarlyPct  int64
	AuctionSellerKeepNormalPct  int64
	AuctionSellerKeepEarlyPct   int64
}

// TopupFeePercentDisplay ค่า % สำหรับแสดงใน UI (เช่น 1.7655)
func (c WalletFeesConfig) TopupFeePercentDisplay() float64 {
	return float64(c.OmisePromptPayFeePPM) / 10000.0
}
