package entity

import (
	"database/sql"
	"time"
)

// CreditActivityRow is one merged row from transactions ∪ bid_transactions.
type CreditActivityRow struct {
	EntryType     string
	CreatedAt     time.Time
	UpdatedAt     sql.NullTime
	ChargeID      sql.NullString
	TopupAmount   sql.NullInt64 // net credit
	TopupPaid     sql.NullInt64 // gross paid
	TopupFee      sql.NullInt64
	Status        sql.NullString
	Paid          sql.NullBool
	Credited      sql.NullBool
	BidTxID       sql.NullInt64
	AuctionID           sql.NullString
	AuctionTitle        sql.NullString
	AuctionCoverImageURL sql.NullString
	LedgerAmount        sql.NullInt64
	BidAmount     sql.NullInt64
	Note          sql.NullString
}
