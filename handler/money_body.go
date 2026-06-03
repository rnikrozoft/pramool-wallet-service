package handler

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rnikrozoft/pramool-wallet-service/internal/money"
)

func parseWholeBahtAmountBody(c *fiber.Ctx) (int64, error) {
	var raw struct {
		Amount json.RawMessage `json:"amount"`
	}
	if err := c.BodyParser(&raw); err != nil {
		return 0, err
	}
	if len(raw.Amount) == 0 {
		return 0, money.ErrInvalidBaht
	}
	return money.UnmarshalJSONInt64Baht(raw.Amount)
}

func parseWholeBahtAmountQuery(c *fiber.Ctx, key string) (int64, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, money.ErrInvalidBaht
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, money.ErrInvalidBaht
	}
	return n, nil
}
