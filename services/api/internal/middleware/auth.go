package middleware

import (
	"log"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkjwt "github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/framed-app/api/pkg/db"
	"github.com/gofiber/fiber/v2"
)

// ClerkAuth returns a Fiber middleware that verifies Clerk JWTs and provisions
// users on first sign-in. It stores the DB user UUID in c.Locals("userID").
func ClerkAuth(pool *db.Pool, secretKey string) fiber.Handler {
	clerk.SetKey(secretKey)
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		token, found := strings.CutPrefix(authHeader, "Bearer ")
		if !found || token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization token"})
		}

		claims, err := clerkjwt.Verify(c.Context(), &clerkjwt.VerifyParams{Token: token})
		if err != nil {
			log.Printf("[auth] jwt verify failed: %v", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}

		clerkID := claims.Subject

		// Provision user on first sign-in; subsequent requests are a no-op update.
		var userID string
		err = pool.QueryRow(c.Context(),
			`INSERT INTO users (clerk_id) VALUES ($1)
			 ON CONFLICT (clerk_id) DO UPDATE SET clerk_id = EXCLUDED.clerk_id
			 RETURNING id`,
			clerkID).Scan(&userID)
		if err != nil {
			log.Printf("[auth] user upsert failed for clerk_id=%s: %v", clerkID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to provision user"})
		}

		c.Locals("userID", userID)
		return c.Next()
	}
}
