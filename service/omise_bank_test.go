package service

import "testing"

func TestOmiseBankBrand(t *testing.T) {
	cases := map[string]string{
		"KBANK": "kbank",
		"SCB":   "scb",
		"CIMBT": "cimb",
	}
	for in, want := range cases {
		got, ok := omiseBankBrand(in)
		if !ok || got != want {
			t.Fatalf("%s: got %q ok=%v want %q", in, got, ok, want)
		}
	}
	if _, ok := omiseBankBrand("UNKNOWN"); ok {
		t.Fatal("expected unknown bank to fail")
	}
}
