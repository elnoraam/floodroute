package services

import (
	"time"

	"github.com/floodroute/floodroute/backend/models"
	"github.com/floodroute/floodroute/backend/repositories"
)

type IncidentService interface {
	ReportIncident(incident *models.Incident) error
	GetActiveIncidents() ([]models.Incident, error)
	GetNearbyIncidents(lat, lon float64, radius float64) ([]models.Incident, error)
}

type incidentService struct {
	repo repositories.IncidentRepository
}

func NewIncidentService(repo repositories.IncidentRepository) IncidentService {
	return &incidentService{repo: repo}
}

func (s *incidentService) ReportIncident(incident *models.Incident) error {
	// Set default expiration (e.g., 4 hours)
	if incident.ExpiresAt.IsZero() {
		incident.ExpiresAt = time.Now().Add(4 * time.Hour)
	}
	return s.repo.Create(incident)
}

func (s *incidentService) GetActiveIncidents() ([]models.Incident, error) {
	// Filter out expired incidents if needed, but for now just get all
	// In a real app, you'd add a where clause for ExpiresAt > now
	return s.repo.GetAll()
}

func (s *incidentService) GetNearbyIncidents(lat, lon float64, radius float64) ([]models.Incident, error) {
	return s.repo.GetNearby(lat, lon, radius)
}
