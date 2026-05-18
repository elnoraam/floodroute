package services

import (
	"fmt"

	"github.com/floodroute/floodroute/backend/models"
	"github.com/floodroute/floodroute/backend/repositories"
)

type RoutingService interface {
	GetSafeRoute(origin, destination string) (*models.RouteResponse, error)
}

type routingService struct {
	incidentRepo repositories.IncidentRepository
}

func NewRoutingService(incidentRepo repositories.IncidentRepository) RoutingService {
	return &routingService{incidentRepo: incidentRepo}
}

func (s *routingService) GetSafeRoute(origin, destination string) (*models.RouteResponse, error) {
	// 1. Get route from OpenRouteService (MOCKED for now)
	// Example GeoJSON LineString from Monas to Bundaran HI
	geoJSON := `{"type":"LineString","coordinates":[[106.8272,-6.1754],[106.8227,-6.1924]]}`
	
	mockRoute := &models.RouteResponse{
		Distance: 2500,
		Duration: 600,
		Geometry: geoJSON,
	}

	// 2. Analyze hazards along the route using PostGIS
	hazards, err := s.incidentRepo.GetByRouteGeometry(geoJSON, 100) // 100m buffer
	if err != nil {
		return nil, fmt.Errorf("failed to analyze hazards: %v", err)
	}

	mockRoute.Hazards = hazards
	
	// 3. Compute Risk Score
	// Simple heuristic: Risk = (Number of Hazards * Average Severity) / Max possible
	if len(hazards) > 0 {
		var totalSeverity int
		for _, h := range hazards {
			totalSeverity += h.Severity
		}
		mockRoute.RiskScore = float64(totalSeverity) / (float64(len(hazards)) * 5.0)
	} else {
		mockRoute.RiskScore = 0
	}
	
	return mockRoute, nil
}
