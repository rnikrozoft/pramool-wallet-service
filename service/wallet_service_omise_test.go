package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestVerifyOmiseSignature_AgainstVector(t *testing.T) {
	key := []byte("test-webhook-hmac-key!!")
	secretB64 := base64.StdEncoding.EncodeToString(key)
	ts := "1758696391"
	body := []byte(`{"object":"event","key":"charge.complete"}`)

	mac := hmac.New(sha256.New, key)
	p := make([]byte, 0, len(ts)+1+len(body))
	p = append(p, ts...)
	p = append(p, '.')
	p = append(p, body...)
	mac.Write(p)
	wantHex := hex.EncodeToString(mac.Sum(nil))

	s := &WalletService{}
	if !s.VerifyOmiseSignature(secretB64, wantHex, ts, body) {
		t.Fatal("expected signature to verify")
	}
	if s.VerifyOmiseSignature(secretB64, wantHex, ts, []byte(`{}`)) {
		t.Fatal("tampered body must not verify")
	}
	if s.VerifyOmiseSignature(secretB64, wantHex, "", body) {
		t.Fatal("empty timestamp must not verify")
	}
}
