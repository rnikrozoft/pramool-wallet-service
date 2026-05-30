package config

import (
	"os"
	"strconv"
	"strings"
)

// WalletFeesConfig ค่านโยบายค่าธรรมเนียมที่แสดงใน UI และใช้คำนวณเครดิตสุทธิ
// ผู้ใช้ทุกคนรับภาระค่า Omise ตอนเติม/ถอน — แพลตฟอร์มเก็บค่าคอมมิชชันตอนปิดประมูลแยกต่างหาก
type WalletFeesConfig struct {
	// MinTopupGrossTHB ยอดชำระ PromptPay ขั้นต่ำต่อครั้ง (บาทเต็ม) ก่อนสร้าง QR
	MinTopupGrossTHB int64

	// MinWithdrawCreditTHB จำนวนเครดิตขั้นต่ำที่ผู้ใช้ขอถอนต่อครั้ง (หักจากกระเป๋า)
	MinWithdrawCreditTHB int64

	// OmisePromptPayFeePPM อัตราค่าธรรมเนียม PromptPay โดยประมาณ ต่อ 1_000_000 สตางค์บาท
	// ค่าเริ่มต้น 17655 = 1.65% × 1.07 (Omise 1.65% + VAT 7% บนค่าธรรมเนียม) ≈ 1.7655%
	OmisePromptPayFeePPM int64

	// OmiseTransferFeeTHB ค่าธรรมเนียมโอนเข้าธนาคารของ Omise ต่อครั้ง (บาทเต็ม โดยประมาณ 20+VAT)
	OmiseTransferFeeTHB int64

	// AuctionPlatformFeeNormalPct ส่วนแบ่งแพลตฟอร์มเมื่อปิดประมูลตามเวลา (% ของราคาปิด)
	AuctionPlatformFeeNormalPct int64

	// AuctionPlatformFeeEarlyPct ส่วนแบ่งแพลตฟอร์มเมื่อผู้ขายปิดก่อนเวลา (% ของราคาปิด)
	AuctionPlatformFeeEarlyPct int64

	// AuctionSellerKeepNormalPct ส่วนที่ผู้ขายได้จากราคาปิด เมื่อปิดตามเวลา (%) — มัก = 100 - platform fee
	AuctionSellerKeepNormalPct int64

	// AuctionSellerKeepEarlyPct ส่วนที่ผู้ขายได้เมื่อปิดก่อนเวลา (%)
	AuctionSellerKeepEarlyPct int64
}

// DefaultWalletFees ค่าเริ่มต้นตาม Omise Thailand list price (พ.ศ. 2568–2569) — ปรับผ่าน env ได้
func DefaultWalletFees() WalletFeesConfig {
	return WalletFeesConfig{
		MinTopupGrossTHB:           100,
		MinWithdrawCreditTHB:       100,
		OmisePromptPayFeePPM:       17655,
		OmiseTransferFeeTHB:        21,
		AuctionPlatformFeeNormalPct: 25,
		AuctionPlatformFeeEarlyPct:  30,
		AuctionSellerKeepNormalPct:  75,
		AuctionSellerKeepEarlyPct:   70,
	}
}

// LoadWalletFeesFromEnv โหลดจากตัวแปรสภาพแวดล้อม (ไม่ตั้ง = ใช้ DefaultWalletFees)
func LoadWalletFeesFromEnv() WalletFeesConfig {
	cfg := DefaultWalletFees()
	cfg.MinTopupGrossTHB = envInt64("WALLET_MIN_TOPUP_GROSS_THB", cfg.MinTopupGrossTHB)
	cfg.MinWithdrawCreditTHB = envInt64("WALLET_MIN_WITHDRAW_CREDIT_THB", cfg.MinWithdrawCreditTHB)
	cfg.OmisePromptPayFeePPM = envInt64("WALLET_OMISE_PROMPTPAY_FEE_PPM", cfg.OmisePromptPayFeePPM)
	cfg.OmiseTransferFeeTHB = envInt64("WALLET_OMISE_TRANSFER_FEE_THB", cfg.OmiseTransferFeeTHB)
	cfg.AuctionPlatformFeeNormalPct = envInt64("AUCTION_PLATFORM_FEE_NORMAL_PCT", cfg.AuctionPlatformFeeNormalPct)
	cfg.AuctionPlatformFeeEarlyPct = envInt64("AUCTION_PLATFORM_FEE_EARLY_PCT", cfg.AuctionPlatformFeeEarlyPct)
	cfg.AuctionSellerKeepNormalPct = envInt64("AUCTION_SELLER_KEEP_NORMAL_PCT", cfg.AuctionSellerKeepNormalPct)
	cfg.AuctionSellerKeepEarlyPct = envInt64("AUCTION_SELLER_KEEP_EARLY_PCT", cfg.AuctionSellerKeepEarlyPct)
	return cfg
}

// TopupFeePercentDisplay ค่า % สำหรับแสดงใน UI (เช่น 1.7655)
func (c WalletFeesConfig) TopupFeePercentDisplay() float64 {
	return float64(c.OmisePromptPayFeePPM) / 10000.0
}

func envInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}
