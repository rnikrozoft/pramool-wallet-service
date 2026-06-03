package entity

import "testing"

func TestWithdrawStatusFromWebhookEvent(t *testing.T) {
	cases := []struct {
		key    string
		want   string
		wantOK bool
	}{
		{"transfer.create", WithdrawStatusProcessing, true},
		{"transfer.send", WithdrawStatusProcessing, true},
		{"transfer.pay", WithdrawStatusCompleted, true},
		{"transfer.fail", WithdrawStatusFailed, true},
		{"transfer.destroy", WithdrawStatusFailed, true},
		{"recipient.verify", "", false},
	}
	for _, c := range cases {
		got, ok := WithdrawStatusFromWebhookEvent(c.key, "")
		if ok != c.wantOK || got != c.want {
			t.Fatalf("%s: got %q ok=%v want %q ok=%v", c.key, got, ok, c.want, c.wantOK)
		}
	}
}
