package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/rnikrozoft/pramool-wallet-service/mapper"
	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
	"github.com/rnikrozoft/pramool-wallet-service/repository"
	"github.com/rnikrozoft/pramool-wallet-service/service"
)

type WalletHandler struct {
	service       *service.WalletService
	webhookSecret string
}

func NewWalletHandler(service *service.WalletService, webhookSecret string) *WalletHandler {
	return &WalletHandler{service: service, webhookSecret: webhookSecret}
}

func (h *WalletHandler) FeeRates(c *fiber.Ctx) error {
	return c.JSON(h.service.FeeRates(c.UserContext()))
}

func (h *WalletHandler) Topup(c *fiber.Ctx) error {
	min := h.service.FeeRates(c.UserContext()).MinTopupGrossTHB
	amount, err := parseWholeBahtAmountBody(c)
	if err != nil || amount < min {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": fmt.Sprintf("จำนวนเงินต้องเป็นบาทเต็ม ขั้นต่ำ %d บาท", min),
		})
	}
	userID, _ := c.Locals("user_id").(string)
	req := dto.TopupRequest{Amount: amount}
	in := mapper.TopupRequestToInput(userID, &req)
	result, err := h.service.CreatePromptPayTopup(c.UserContext(), in)
	if err != nil {
		if errors.Is(err, repository.ErrTopupBanned) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "บัญชีถูกจำกัด ไม่สามารถเติมเงินได้"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	return c.JSON(mapper.TopupResultToResponse(result))
}

func (h *WalletHandler) TopupStatus(c *fiber.Ctx) error {
	chargeID := strings.TrimSpace(c.Query("charge_id"))
	if chargeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ต้องระบุ charge_id"})
	}
	userID, _ := c.Locals("user_id").(string)
	result, err := h.service.SyncTopupChargeStatus(c.UserContext(), userID, chargeID)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "ไม่พบรายการ") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": msg})
		}
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"message": msg})
	}
	return c.JSON(mapper.TopupStatusToResponse(result))
}

func (h *WalletHandler) PendingTopup(c *fiber.Ctx) error {
	min := h.service.FeeRates(c.UserContext()).MinTopupGrossTHB
	amount, err := parseWholeBahtAmountQuery(c, "amount")
	if err != nil || amount < min {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": fmt.Sprintf("จำนวนเงินต้องเป็นบาทเต็ม ขั้นต่ำ %d บาท", min),
		})
	}
	userID, _ := c.Locals("user_id").(string)
	result, ok, err := h.service.TryResumePendingTopup(c.UserContext(), userID, amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	if !ok || result == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.JSON(mapper.TopupResultToResponse(result))
}

func (h *WalletHandler) Withdraw(c *fiber.Ctx) error {
	min := h.service.FeeRates(c.UserContext()).MinWithdrawCreditTHB
	amount, err := parseWholeBahtAmountBody(c)
	if err != nil || amount < min {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": fmt.Sprintf("จำนวนเงินต้องเป็นบาทเต็ม ขั้นต่ำ %d บาท", min),
		})
	}
	userID, _ := c.Locals("user_id").(string)
	req := dto.WithdrawRequest{Amount: amount}
	in := mapper.WithdrawRequestToInput(userID, &req)
	result, err := h.service.WithdrawCredit(c.UserContext(), in)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrCreditDebt):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"message": "คุณมียอดค้างชำระ กรุณาเติมเครดิตให้ครบก่อนถอนเงิน"})
		case errors.Is(err, repository.ErrInsufficientCredit):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "เครดิตไม่พอ"})
		case errors.Is(err, repository.ErrMissingBankAccount):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "กรุณาบันทึกบัญชีธนาคารในโปรไฟล์ก่อนถอนเงิน"})
		case errors.Is(err, repository.ErrWithdrawalBlocked):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"message": err.Error()})
		case errors.Is(err, repository.ErrWithdrawBanned):
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "บัญชีถูกระงับ ไม่สามารถถอนเงินได้"})
		default:
			msg := err.Error()
			if strings.Contains(msg, "withdrawal blocked:") {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"message": strings.TrimPrefix(msg, "withdrawal blocked: ")})
			}
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"message": msg})
		}
	}
	return c.JSON(mapper.WithdrawResultToResponse(result))
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
	if filter != "topup" && filter != "auction" && filter != "withdraw" {
		filter = "all"
	}
	sortKey := strings.TrimSpace(c.Query("sort"))
	sortOrder := strings.TrimSpace(c.Query("order"))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	rows, total, err := h.service.ListCreditActivity(userID, limit, offset, filter, sortKey, sortOrder)
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

	// Same endpoint for top-up (charge.*) and withdraw (transfer.*) — route by event key.
	if transfer, ok := mapper.WebhookPayloadToTransfer(payload); ok {
		// transfer.destroy removes the object from Omise — GET /transfers/:id returns 404.
		skipAPIVerify := transfer.EventKey == "transfer.destroy"
		if signature == "" && !skipAPIVerify {
			st, err := h.service.FetchTransferStateFromAPI(c.UserContext(), transfer.TransferID)
			if err != nil {
				return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
					"message": err.Error(),
					"hint":    "Configure OMISE_WEBHOOK_SECRET or ensure OMISE_SECRET_KEY can GET /transfers for event verification",
				})
			}
			transfer.Status = st
		}
		if err := h.service.ProcessWebhookTransfer(c.UserContext(), transfer); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
		}
		return c.SendStatus(fiber.StatusOK)
	}

	if dispute, ok := mapper.WebhookPayloadToDispute(payload); ok {
		if signature == "" {
			st, chargeID, err := h.service.FetchDisputeStateFromAPI(c.UserContext(), dispute.DisputeID)
			if err != nil {
				return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
					"message": err.Error(),
					"hint":    "Configure OMISE_WEBHOOK_SECRET or ensure OMISE_SECRET_KEY can GET /disputes for event verification",
				})
			}
			dispute.Status = st
			if dispute.ChargeID == "" {
				dispute.ChargeID = chargeID
			}
		}
		if err := h.service.ProcessWebhookDispute(c.UserContext(), dispute); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
		}
		return c.SendStatus(fiber.StatusOK)
	}

	charge, ok := mapper.WebhookPayloadToCharge(payload)
	if !ok {
		return c.SendStatus(fiber.StatusOK)
	}

	// Unsigned: Omise omits signature headers until you add a webhook secret in Test/Live webhooks settings.
	// Per https://docs.omise.co/api-webhooks#protecting-your-endpoints use event verification (GET charge) instead.
	if signature == "" {
		st, paid, err := h.service.FetchChargeStateFromAPI(c.UserContext(), charge.ChargeID)
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

