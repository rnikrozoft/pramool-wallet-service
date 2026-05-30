package mapper

import "testing"

func TestWebhookPayloadToTransfer_transferPay(t *testing.T) {
	payload := map[string]any{
		"object": "event",
		"key":    "transfer.pay",
		"data": map[string]any{
			"object": "transfer",
			"id":     "trsf_test_abc",
			"status": "paid",
		},
	}
	tr, ok := WebhookPayloadToTransfer(payload)
	if !ok || tr.TransferID != "trsf_test_abc" || tr.EventKey != "transfer.pay" {
		t.Fatalf("got %+v ok=%v", tr, ok)
	}
}

func TestWebhookPayloadToCharge_skipsTransferEvent(t *testing.T) {
	payload := map[string]any{
		"object": "event",
		"key":    "transfer.pay",
		"data": map[string]any{
			"object": "transfer",
			"id":     "trsf_test_abc",
		},
	}
	if _, ok := WebhookPayloadToCharge(payload); ok {
		t.Fatal("charge mapper should not handle transfer events")
	}
}
