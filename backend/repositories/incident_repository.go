package repositories

import (
	"github.com/floodroute/floodroute/backend/db"
	"github.com/floodroute/floodroute/backend/models"
)

type IncidentRepository interface {
	Create(incident *models.Incident) error
	GetAll() ([]models.Incident, error)
	GetNearby(lat, lon float64, radius float64) ([]models.Incident, error)
	GetByID(id uint) (*models.Incident, error)
	GetByRouteGeometry(geoJSON string, buffer float64) ([]models.Incident, error)
	Delete(id uint) error
}

type incidentRepository struct{}

func NewIncidentRepository() IncidentRepository {
	return &incidentRepository{}
}

func (r *incidentRepository) Create(incident *models.Incident) error {
	return db.DB.Create(incident).Error
}

func (r *incidentRepository) GetAll() ([]models.Incident, error) {
	var incidents []models.Incident
	err := db.DB.Find(&incidents).Error
	return incidents, err
}

func (r *incidentRepository) GetNearby(lat, lon float64, radius float64) ([]models.Incident, error) {
	var incidents []models.Incident
	// radius in meters
	err := db.DB.Where("ST_DWithin(location, ST_SetSRID(ST_MakePoint(?, ?), 4326), ?)", lon, lat, radius).Find(&incidents).Error
	return incidents, err
}

func (r *incidentRepository) GetByID(id uint) (*models.Incident, error) {
	var incident models.Incident
	err := db.DB.First(&incident, id).Error
	return &incident, err
}

func (r *incidentRepository) GetByRouteGeometry(geoJSON string, buffer float64) ([]models.Incident, error) {
	var incidents []models.Incident
	// geoJSON is the route geometry in GeoJSON format
	// buffer is the distance in meters to check around the route
	err := db.DB.Where("ST_Intersects(location, ST_Buffer(ST_GeomFromGeoJSON(?)::geography, ?)::geometry)", geoJSON, buffer).Find(&incidents).Error
	return incidents, err
}

func (r *incidentRepository) Delete(id uint) error {
	return db.DB.Delete(&models.Incident{}, id).Error
}
