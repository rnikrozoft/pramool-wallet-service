package handler

import (
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

func (h *WalletHandler) OmiseWebhook(c *fiber.Ctx) error {
	if h.webhookSecret == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "missing webhook secret"})
	}

	signature := strings.TrimSpace(c.Get("omise-signature"))
	if signature == "" {
		signature = strings.TrimSpace(c.Get("x-omise-signature"))
	}
	timestamp := c.Get("omise-signature-timestamp")
	body := c.Body()
	if !h.service.VerifyOmiseSignature(h.webhookSecret, signature, timestamp, body) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid webhook signature"})
	}

	var payload map[string]any
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid payload"})
	}

	charge, ok := mapper.WebhookPayloadToCharge(payload)
	if !ok {
		return c.SendStatus(fiber.StatusOK)
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
