package main

import (
	"log"

	"github.com/floodroute/floodroute/backend/db"
	"github.com/floodroute/floodroute/backend/handlers"
	"github.com/floodroute/floodroute/backend/repositories"
	"github.com/floodroute/floodroute/backend/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	db.InitDB()

	// Initialize repositories, services, and handlers
	incidentRepo := repositories.NewIncidentRepository()
	incidentService := services.NewIncidentService(incidentRepo)
	incidentHandler := handlers.NewIncidentHandler(incidentService)

	routingService := services.NewRoutingService(incidentRepo)
	routingHandler := handlers.NewRoutingHandler(routingService)

	app := fiber.New()
	app.Use(logger.New())

	api := app.Group("/api")
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "OK"})
	})

	incidents := api.Group("/incidents")
	incidents.Post("/", incidentHandler.ReportIncident)
	incidents.Get("/", incidentHandler.GetIncidents)

	routes := api.Group("/routes")
	routes.Get("/", routingHandler.GetRoute)

	log.Fatal(app.Listen(":8080"))
}
