package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rnikrozoft/pramool-wallet-service/mapper"
	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
	"github.com/rnikrozoft/pramool-wallet-service/service"
)

type WalletHandler struct {
	service       *service.WalletService
	webhookSecret string
}

func NewWalletHandler(service *service.WalletService, webhookSecret string) *WalletHandler {
	return &WalletHandler{service: service, webhookSecret: webhookSecret}
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
	items, err := h.service.ListTransactions(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	return c.JSON(mapper.TransactionsToListResponse(items))
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
