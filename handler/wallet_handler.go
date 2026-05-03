package handler

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rnikrozoft/pramool-wallet-service/mapper"
	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
	"github.com/rnikrozoft/pramool-wallet-service/service"
)

type WalletHandler struct {
	service       *service.WalletService
	webhookSecret string
	internalKey   string
}

func NewWalletHandler(service *service.WalletService, webhookSecret, internalKey string) *WalletHandler {
	return &WalletHandler{service: service, webhookSecret: webhookSecret, internalKey: strings.TrimSpace(internalKey)}
}

func (h *WalletHandler) Topup(c *fiber.Ctx) error {
	var req dto.TopupRequest
	if err := c.BodyParser(&req); err != nil || req.Amount < 20 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid amount"})
	}
	userID, _ := c.Locals("user_id").(string)
	in := mapper.TopupRequestToInput(userID, &req)
	result, err := h.service.CreatePromptPayTopup(in)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	return c.JSON(mapper.TopupResultToResponse(result))
}

func (h *WalletHandler) Transactions(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	limit := 20
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 500000 {
			offset = n
		}
	}
	filter := strings.TrimSpace(c.Query("filter"))
	if filter != "topup" && filter != "auction" {
		filter = "all"
	}
	rows, total, err := h.service.ListCreditActivity(userID, limit, offset, filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	return c.JSON(mapper.CreditActivityRowsToResponse(rows, total, limit, offset))
}

func omiseWebhookHeaders(c *fiber.Ctx) (signature string, timestamp string) {
	h := &c.Request().Header
	if v := h.Peek("Omise-Signature"); len(v) > 0 {
		signature = string(v)
	} else if v := h.Peek("X-Omise-Signature"); len(v) > 0 {
		signature = string(v)
	}
	if v := h.Peek("Omise-Signature-Timestamp"); len(v) > 0 {
		timestamp = string(v)
	}
	return strings.TrimSpace(signature), strings.TrimSpace(timestamp)
}

func (h *WalletHandler) OmiseWebhook(c *fiber.Ctx) error {
	body := c.Body()
	signature, timestamp := omiseWebhookHeaders(c)

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid payload"})
	}

	// Signed webhooks (Dashboard webhook secret configured — headers Omise-Signature + Omise-Signature-Timestamp).
	if signature != "" {
		if h.webhookSecret == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "OMISE_WEBHOOK_SECRET is required when Omise sends Omise-Signature",
			})
		}
		if timestamp == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "missing Omise-Signature-Timestamp (required together with Omise-Signature)",
			})
		}
		if !h.service.VerifyOmiseSignature(h.webhookSecret, signature, timestamp, body) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid webhook signature"})
		}
	}

	charge, ok := mapper.WebhookPayloadToCharge(payload)
	if !ok {
		return c.SendStatus(fiber.StatusOK)
	}

	// Unsigned: Omise omits signature headers until you add a webhook secret in Test/Live webhooks settings.
	// Per https://docs.omise.co/api-webhooks#protecting-your-endpoints use event verification (GET charge) instead.
	if signature == "" {
		st, paid, err := h.service.FetchChargeStateFromAPI(charge.ChargeID)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
				"message": err.Error(),
				"hint":    "Either configure a webhook signing secret in Omise Dashboard (recommended), or ensure OMISE_SECRET_KEY can GET /charges for event verification",
			})
		}
		charge.Status = st
		charge.Paid = paid
	}

	if err := h.service.ProcessWebhookCharge(charge.ChargeID, charge.Status, charge.Paid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	return c.SendStatus(fiber.StatusOK)
}

func (h *WalletHandler) ChargeAuctionCloseFee(c *fiber.Ctx) error {
	if h.internalKey == "" || strings.TrimSpace(c.Get("X-Internal-Key")) != h.internalKey {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "unauthorized internal call"})
	}
	var req dto.AuctionCloseFeeRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.SellerID) == "" || req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid request"})
	}
	creditDeduct := req.Amount
	if req.CreditDeduct != nil {
		creditDeduct = *req.CreditDeduct
	}
	if err := h.service.ChargeAuctionCloseFee(req.SellerID, req.AuctionID, req.Amount, creditDeduct); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "fee charged"})
}
