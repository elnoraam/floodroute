package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex;not null" json:"username"`
	Password  string         `gorm:"not null" json:"-"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type IncidentType string

const (
	IncidentFlood       IncidentType = "flood"
	IncidentAccident    IncidentType = "accident"
	IncidentTraffic     IncidentType = "traffic"
	IncidentRoadClosure IncidentType = "road_closure"
	IncidentFallenTree  IncidentType = "fallen_tree"
)

type Incident struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Type        IncidentType   `gorm:"not null" json:"type"`
	Severity    int            `gorm:"default:1" json:"severity"` // 1-5
	Description string         `json:"description"`
	Latitude    float64        `gorm:"not null" json:"latitude"`
	Longitude   float64        `gorm:"not null" json:"longitude"`
	Location    string         `gorm:"type:geometry(Point,4326);index:,type:gist" json:"-"` // PostGIS point
	UserID      *uint          `json:"user_id"`
	User        User           `json:"user"`
	Votes       int            `gorm:"default:1" json:"votes"`
	ExpiresAt   time.Time      `json:"expires_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (i *Incident) BeforeSave(tx *gorm.DB) (err error) {
	i.Location = "SRID=4326;POINT(" + fmt.Sprintf("%f %f", i.Longitude, i.Latitude) + ")"
	return
}

type RouteResponse struct {
	Distance  float64 `json:"distance"`
	Duration  float64 `json:"duration"`
	Geometry  string  `json:"geometry"`
	RiskScore float64 `json:"risk_score"`
	Hazards   []Incident `json:"hazards"`
}
