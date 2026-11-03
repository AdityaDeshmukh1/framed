package handlers

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// BasicsRequest is the tap-only step of onboarding.
type BasicsRequest struct {
	Name    string   `json:"name"`
	Age     int      `json:"age"`
	Seeking []string `json:"seeking"` // any of: women, men, nonbinary
	City    string   `json:"city"`
}

var seekingValues = map[string]bool{"women": true, "men": true, "nonbinary": true}

// SaveBasics serves PUT /me/basics.
//
// Matching doesn't filter on these yet; they're collected now so the beta
// can turn filtering on without asking everyone again.
func (h *Handlers) SaveBasics(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	var req BasicsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 40 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name must be 1–40 characters"})
	}
	if req.Age < 18 || req.Age > 99 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "age must be 18–99"})
	}
	if len(req.Seeking) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "seeking is required"})
	}
	for _, s := range req.Seeking {
		if !seekingValues[s] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "seeking must be women, men or nonbinary"})
		}
	}
	city := strings.TrimSpace(req.City)
	if len(city) > 80 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "city is too long"})
	}

	if _, err := h.pool.Exec(c.Context(),
		`UPDATE users SET display_name = $2, age = $3, seeking = $4, location_city = $5 WHERE id = $1`,
		userID, name, req.Age, req.Seeking, city); err != nil {
		log.Printf("[basics] user=%s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
