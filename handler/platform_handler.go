package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rnikrozoft/pramool-wallet-service/model/dto"
	"github.com/rnikrozoft/pramool-wallet-service/service"
)

type PlatformHandler struct {
	service           *service.WalletService
	platformRecipient string
}

func NewPlatformHandler(svc *service.WalletService, platformRecipient string) *PlatformHandler {
	return &PlatformHandler{
		service:           svc,
		platformRecipient: platformRecipient,
	}
}

func (h *PlatformHandler) Revenue(c *fiber.Ctx) error {
	resp, err := h.service.PlatformRevenue(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	return c.JSON(resp)
}

func (h *PlatformHandler) Withdraw(c *fiber.Ctx) error {
	req := new(dto.PlatformWithdrawRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid body"})
	}
	if req.AdminID < 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "admin_id required"})
	}
	resp, err := h.service.PlatformWithdraw(c.UserContext(), req.AdminID, req.AmountBaht, h.platformRecipient)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
	}
	return c.JSON(resp)
}

func (h *PlatformHandler) ListWithdrawals(c *fiber.Ctx) error {
	resp, err := h.service.ListPlatformWithdrawals(c.UserContext(), 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}
	return c.JSON(resp)
}
