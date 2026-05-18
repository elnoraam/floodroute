package handlers

import (
	"github.com/floodroute/floodroute/backend/services"
	"github.com/gofiber/fiber/v2"
)

type RoutingHandler struct {
	service services.RoutingService
}

func NewRoutingHandler(service services.RoutingService) *RoutingHandler {
	return &RoutingHandler{service: service}
}

func (h *RoutingHandler) GetRoute(c *fiber.Ctx) error {
	origin := c.Query("origin")
	destination := c.Query("destination")

	if origin == "" || destination == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Origin and destination are required"})
	}

	route, err := h.service.GetSafeRoute(origin, destination)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(route)
}
