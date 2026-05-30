package entity

// UserPayoutProfile is bank + credit data needed to send an Omise transfer.
type UserPayoutProfile struct {
	UserID            string
	Credit            int64
	BankID            int64
	BankCode          string
	BankAccountName   string
	BankAccountNumber string
	OmiseRecipientID  string
}

// WithdrawInput is a credit withdrawal request.
type WithdrawInput struct {
	UserID string
	Amount int64
}

// WithdrawResult is returned after a withdrawal is submitted to Omise.
type WithdrawResult struct {
	WithdrawalID      int64
	Amount            int64 // credit deducted
	FeeAmount         int64
	TransferAmount    int64 // approximate THB to bank
	Status            string
	OmiseTransferID   string
	BalanceAfter      int64
	BankAccountName   string
	BankAccountNumber string
	BankCode          string
}
