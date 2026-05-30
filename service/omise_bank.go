package service

import "strings"

// omiseBankBrand maps internal banks.bank_code to Omise recipient bank_account[brand].
// See https://docs.omise.co/supported-banks/thailand
func omiseBankBrand(bankCode string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(bankCode)) {
	case "KBANK":
		return "kbank", true
	case "KTB":
		return "ktb", true
	case "BBL":
		return "bbl", true
	case "SCB":
		return "scb", true
	case "BAY":
		return "bay", true
	case "TTB":
		return "ttb", true
	case "GSB":
		return "gsb", true
	case "BAAC":
		return "baac", true
	case "CIMBT":
		return "cimb", true
	case "UOB":
		return "uob", true
	default:
		return "", false
	}
}
