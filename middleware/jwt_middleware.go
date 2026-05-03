package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type Middleware struct {
	JWTSecret string
}

func (m Middleware) JWTMiddleware(c *fiber.Ctx) error {
	tokenValue := c.Cookies("access_token")
	if tokenValue == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "missing token"})
	}

	token, err := jwt.Parse(tokenValue, func(token *jwt.Token) (interface{}, error) {
		return []byte(m.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid token"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid token"})
	}
	if tu, _ := claims["token_use"].(string); tu == "refresh" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "use access token"})
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		if v, ok := claims["user_id"].(string); ok {
			sub = v
		} else if v, ok := claims["UserID"].(string); ok {
			sub = v
		}
	}
	if sub == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid token"})
	}
	c.Locals("user_id", sub)
	return c.Next()
}
