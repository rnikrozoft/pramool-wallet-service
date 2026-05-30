package entity

// Withdrawal status codes stored in withdrawals.status
const (
	WithdrawStatusPendingReview = "pending_review" // รอการตรวจสอบ
	WithdrawStatusProcessing    = "processing"     // กำลังดำเนินการ
	WithdrawStatusCompleted     = "completed"    // ดำเนินการเสร็จสิ้น
	WithdrawStatusFailed        = "failed"       // ไม่สำเร็จ (คืนเครดิต)
)

// WithdrawStatusLabelTH returns a Thai label for API / history display.
func WithdrawStatusLabelTH(status string) string {
	switch status {
	case WithdrawStatusPendingReview:
		return "รอการตรวจสอบ"
	case WithdrawStatusProcessing:
		return "กำลังดำเนินการ"
	case WithdrawStatusCompleted:
		return "ดำเนินการเสร็จสิ้น"
	case WithdrawStatusFailed:
		return "ไม่สำเร็จ"
	case "paid": // legacy rows
		return "ดำเนินการเสร็จสิ้น"
	case "submitted", "sent", "pending":
		return "กำลังดำเนินการ"
	default:
		if status == "" {
			return "ถอนเครดิต"
		}
		return status
	}
}

// WithdrawStatusFromOmiseTransfer maps Omise transfer.status right after create.
func WithdrawStatusFromOmiseTransfer(omiseStatus string) string {
	switch omiseStatus {
	case "paid":
		return WithdrawStatusCompleted
	case "failed":
		return WithdrawStatusFailed
	case "sent":
		return WithdrawStatusProcessing
	default:
		return WithdrawStatusPendingReview
	}
}

// WithdrawStatusFromWebhookEvent maps Omise event key (+ optional transfer status) to our status.
func WithdrawStatusFromWebhookEvent(eventKey, omiseStatus string) (string, bool) {
	switch eventKey {
	case "transfer.pay":
		return WithdrawStatusCompleted, true
	case "transfer.fail":
		return WithdrawStatusFailed, true
	case "transfer.create", "transfer.send", "transfer.update":
		return WithdrawStatusProcessing, true
	default:
		return "", false
	}
}
