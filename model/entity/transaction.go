package entity

import "time"

// Transaction is a persisted wallet top-up row (Omise charge lifecycle).
type Transaction struct {
	ChargeID  string
	Amount    int64
	Status    string
	Paid      bool
	Credited  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    string
}
