package repository

import "strings"

func normalizeCreditActivitySort(sortKey, sortOrder string) (key, order string) {
	key = strings.ToLower(strings.TrimSpace(sortKey))
	order = strings.ToLower(strings.TrimSpace(sortOrder))
	if order != "asc" {
		order = "desc"
	}
	switch key {
	case "created_at", "entry_type", "amount", "status":
		return key, order
	default:
		return "created_at", "desc"
	}
}

func creditActivityOrderSQL(sortKey, sortOrder, filter string) string {
	key, order := normalizeCreditActivitySort(sortKey, sortOrder)
	dir := strings.ToUpper(order)
	tie := ", created_at DESC"
	if key == "created_at" {
		tie = ""
	}

	switch key {
	case "entry_type":
		return entryTypeOrderExpr(filter) + " " + dir + tie
	case "amount":
		return amountOrderExpr(filter) + " " + dir + tie
	case "status":
		return statusOrderExpr(filter) + " " + dir + tie
	default:
		if order == "asc" {
			return "created_at ASC"
		}
		return "created_at DESC"
	}
}

func entryTypeOrderExpr(filter string) string {
	switch filter {
	case "topup":
		return "'topup'"
	case "withdraw":
		return "'withdraw'"
	case "auction":
		return "bt.tx_type::text"
	default:
		return "entry_type"
	}
}

func amountOrderExpr(filter string) string {
	switch filter {
	case "topup":
		return "t.credit_amount"
	case "auction":
		return "bt.amount"
	case "withdraw":
		return "w.amount"
	default:
		return "COALESCE(ledger_amount, credit_amount, 0)"
	}
}

func statusOrderExpr(filter string) string {
	switch filter {
	case "topup":
		return "COALESCE(t.status, '')"
	case "withdraw":
		return "COALESCE(w.status, '')"
	case "auction":
		return "''"
	default:
		return "COALESCE(status, '')"
	}
}
