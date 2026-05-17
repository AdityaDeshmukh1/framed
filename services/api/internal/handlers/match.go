package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

type matchRequest struct {
	UserID string `json:"user_id"`
}

// Match finds the top compatible users for the given user_id, computes a
// first-date film for each pair, and returns the persisted match rows.
func (h *Handlers) Match(c *fiber.Ctx) error {
	var req matchRequest
	if err := c.BodyParser(&req); err != nil || req.UserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id is required",
		})
	}

	log.Printf("[match] finding matches for user=%s", req.UserID)

	matches, err := h.matcher.FindMatches(c.Context(), req.UserID, 20)
	if err != nil {
		log.Printf("[match] error for user=%s err=%v", req.UserID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to compute matches",
		})
	}

	log.Printf("[match] found %d matches for user=%s", len(matches), req.UserID)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"matches": matches,
		"count":   len(matches),
	})
}

