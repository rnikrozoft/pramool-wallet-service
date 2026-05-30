package money

import "testing"

func TestUnmarshalJSONInt64Baht(t *testing.T) {
	n, err := UnmarshalJSONInt64Baht([]byte(`500`))
	if err != nil || n != 500 {
		t.Fatalf("got %d %v", n, err)
	}
	if _, err := UnmarshalJSONInt64Baht([]byte(`99.9`)); err == nil {
		t.Fatal("expected error for decimal")
	}
}
