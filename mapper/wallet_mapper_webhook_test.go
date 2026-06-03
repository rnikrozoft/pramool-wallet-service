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

func TestWebhookPayloadToTransfer_transferDestroy(t *testing.T) {
	payload := map[string]any{
		"object": "event",
		"key":    "transfer.destroy",
		"data": map[string]any{
			"object":  "transfer",
			"id":      "trsf_test_abc",
			"deleted": true,
		},
	}
	tr, ok := WebhookPayloadToTransfer(payload)
	if !ok || tr.TransferID != "trsf_test_abc" || tr.EventKey != "transfer.destroy" {
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

func TestWebhookPayloadToDispute_create(t *testing.T) {
	payload := map[string]any{
		"object": "event",
		"key":    "dispute.create",
		"data": map[string]any{
			"object": "dispute",
			"id":     "dspt_test_abc",
			"status": "open",
			"charge": "chrg_test_xyz",
		},
	}
	d, ok := WebhookPayloadToDispute(payload)
	if !ok || d.DisputeID != "dspt_test_abc" || d.ChargeID != "chrg_test_xyz" || d.EventKey != "dispute.create" {
		t.Fatalf("got %+v ok=%v", d, ok)
	}
}

func TestWebhookPayloadToCharge_skipsDisputeEvent(t *testing.T) {
	payload := map[string]any{
		"object": "event",
		"key":    "dispute.create",
		"data": map[string]any{
			"object": "dispute",
			"id":     "dspt_test_abc",
			"charge": "chrg_test_xyz",
		},
	}
	if _, ok := WebhookPayloadToCharge(payload); ok {
		t.Fatal("charge mapper should not handle dispute events")
	}
}
