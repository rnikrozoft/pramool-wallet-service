package money

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrNotWholeBaht = errors.New("amount must be whole baht without decimals")
	ErrInvalidBaht  = errors.New("invalid baht amount")
)

func ValidatePositiveBaht(amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("%w: must be positive", ErrInvalidBaht)
	}
	return nil
}

func UnmarshalJSONInt64Baht(data []byte) (int64, error) {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, ErrInvalidBaht
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return 0, err
	}
	ns := strings.TrimSpace(num.String())
	if strings.ContainsAny(ns, ".") || strings.ContainsAny(ns, "eE") {
		return 0, ErrNotWholeBaht
	}
	n, err := strconv.ParseInt(ns, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidBaht, err)
	}
	return n, nil
}
