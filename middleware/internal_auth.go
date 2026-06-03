package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func InternalAuth(secret string) fiber.Handler {
	secret = strings.TrimSpace(secret)
	return func(c *fiber.Ctx) error {
		if secret == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "INTERNAL_API_SECRET not configured",
			})
		}
		if strings.TrimSpace(c.Get("X-Internal-Secret")) != secret {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "unauthorized"})
		}
		return c.Next()
	}
}
