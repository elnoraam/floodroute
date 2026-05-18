package handlers

import (
	"strconv"

	"github.com/floodroute/floodroute/backend/models"
	"github.com/floodroute/floodroute/backend/services"
	"github.com/gofiber/fiber/v2"
)

type IncidentHandler struct {
	service services.IncidentService
}

func NewIncidentHandler(service services.IncidentService) *IncidentHandler {
	return &IncidentHandler{service: service}
}

func (h *IncidentHandler) ReportIncident(c *fiber.Ctx) error {
	incident := new(models.Incident)
	if err := c.BodyParser(incident); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	if err := h.service.ReportIncident(incident); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(incident)
}

func (h *IncidentHandler) GetIncidents(c *fiber.Ctx) error {
	latStr := c.Query("lat")
	lonStr := c.Query("lon")
	radiusStr := c.Query("radius")

	if latStr != "" && lonStr != "" {
		lat, _ := strconv.ParseFloat(latStr, 64)
		lon, _ := strconv.ParseFloat(lonStr, 64)
		radius, _ := strconv.ParseFloat(radiusStr, 64)
		if radius == 0 {
			radius = 5000 // default 5km
		}

		incidents, err := h.service.GetNearbyIncidents(lat, lon, radius)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(incidents)
	}

	incidents, err := h.service.GetActiveIncidents()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(incidents)
}
