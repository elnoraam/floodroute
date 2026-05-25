package model

import (
	"time"
)

// ─── User ─────────────────────────────────────────────────────────────────────

type Role string

const (
	RoleSuperadmin Role = "SUPERADMIN"
	RoleProducer   Role = "PRODUCER"
	RoleConsumer   Role = "CONSUMER"
)

type User struct {
	ID           int64     `db:"id"            json:"id"`
	Username     string    `db:"username"      json:"username"`
	Email        string    `db:"email"         json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	DisplayName  *string   `db:"display_name"  json:"displayName,omitempty"`
	Role         Role      `db:"role"          json:"role"`
	IsActive     bool      `db:"is_active"     json:"isActive"`
	CreatedAt    time.Time `db:"created_at"    json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updatedAt"`
}

// ─── Incident ─────────────────────────────────────────────────────────────────

type IncidentType string

const (
	IncidentFlood       IncidentType = "FLOOD"
	IncidentAccident    IncidentType = "ACCIDENT"
	IncidentCongestion  IncidentType = "CONGESTION"
	IncidentRoadClosure IncidentType = "ROAD_CLOSURE"
	IncidentFallenTree  IncidentType = "FALLEN_TREE"
	IncidentOther       IncidentType = "OTHER"
)

type Incident struct {
	ID          int64        `db:"id"          json:"id"`
	UserID      *int64       `db:"user_id"     json:"userId,omitempty"`
	Type        IncidentType `db:"type"        json:"type"`
	Severity    int16        `db:"severity"    json:"severity"`
	Title       *string      `db:"title"       json:"title,omitempty"`
	Description *string      `db:"description" json:"description,omitempty"`
	ImageURL    *string      `db:"image_url"   json:"imageUrl,omitempty"`
	Latitude    float64      `db:"latitude"    json:"latitude"`
	Longitude   float64      `db:"longitude"   json:"longitude"`
	Upvotes     int          `db:"upvotes"     json:"upvotes"`
	IsVerified  bool         `db:"is_verified" json:"isVerified"`
	IsActive    bool         `db:"is_active"   json:"isActive"`
	ExpiresAt   time.Time    `db:"expires_at"  json:"expiresAt"`
	ReportedAt  time.Time    `db:"reported_at" json:"reportedAt"`
	UpdatedAt   time.Time    `db:"updated_at"  json:"updatedAt"`
}

// ─── FloodZone ────────────────────────────────────────────────────────────────

type FloodZoneSource string

const (
	SourceComputed   FloodZoneSource = "COMPUTED"
	SourceHistorical FloodZoneSource = "HISTORICAL"
	SourceBMKG       FloodZoneSource = "BMKG"
)

type FloodZone struct {
	ID            int64           `db:"id"             json:"id"`
	Name          *string         `db:"name"           json:"name,omitempty"`
	Source        FloodZoneSource `db:"source"         json:"source"`
	RiskLevel     int16           `db:"risk_level"     json:"riskLevel"`
	GeomWKT       string          `db:"geom_wkt"       json:"-"`
	RainfallMm    *float64        `db:"rainfall_mm"    json:"rainfallMm,omitempty"`
	IncidentCount int             `db:"incident_count" json:"incidentCount"`
	ValidFrom     time.Time       `db:"valid_from"     json:"validFrom"`
	ValidUntil    *time.Time      `db:"valid_until"    json:"validUntil,omitempty"`
	CreatedAt     time.Time       `db:"created_at"     json:"createdAt"`
	// GeoJSON polygon coordinates (populated in service layer)
	Coordinates [][][2]float64 `db:"-" json:"coordinates,omitempty"`
}

// ─── WeatherCache ─────────────────────────────────────────────────────────────

type WeatherCache struct {
	ID            int64      `db:"id"             json:"id"`
	Source        string     `db:"source"         json:"source"`
	StationID     *string    `db:"station_id"     json:"stationId,omitempty"`
	Latitude      *float64   `db:"latitude"       json:"latitude,omitempty"`
	Longitude     *float64   `db:"longitude"      json:"longitude,omitempty"`
	Temperature   *float64   `db:"temperature"    json:"temperature,omitempty"`
	Humidity      *float64   `db:"humidity"       json:"humidity,omitempty"`
	Rainfall1h    *float64   `db:"rainfall_1h"    json:"rainfall1h,omitempty"`
	Rainfall3h    *float64   `db:"rainfall_3h"    json:"rainfall3h,omitempty"`
	Rainfall24h   *float64   `db:"rainfall_24h"   json:"rainfall24h,omitempty"`
	WindSpeed     *float64   `db:"wind_speed"     json:"windSpeed,omitempty"`
	WindDirection *float64   `db:"wind_direction" json:"windDirection,omitempty"`
	ConditionCode *string    `db:"condition_code" json:"conditionCode,omitempty"`
	ConditionText *string    `db:"condition_text" json:"conditionText,omitempty"`
	RawData       *string    `db:"raw_data"       json:"-"`
	FetchedAt     time.Time  `db:"fetched_at"     json:"fetchedAt"`
	ValidUntil    *time.Time `db:"valid_until"    json:"validUntil,omitempty"`
}

// ─── Route ────────────────────────────────────────────────────────────────────

type Route struct {
	ID             int64      `db:"id"              json:"id"`
	UserID         *int64     `db:"user_id"         json:"userId,omitempty"`
	OriginLat      float64    `db:"origin_lat"      json:"originLat"`
	OriginLon      float64    `db:"origin_lon"      json:"originLon"`
	DestLat        float64    `db:"dest_lat"        json:"destLat"`
	DestLon        float64    `db:"dest_lon"        json:"destLon"`
	OriginName     *string    `db:"origin_name"     json:"originName,omitempty"`
	DestName       *string    `db:"dest_name"       json:"destName,omitempty"`
	DistanceM      *float64   `db:"distance_m"      json:"distanceM,omitempty"`
	DurationS      *float64   `db:"duration_s"      json:"durationS,omitempty"`
	SafetyScore    *float64   `db:"safety_score"    json:"safetyScore,omitempty"`
	CongestionProb *float64   `db:"congestion_prob" json:"congestionProb,omitempty"`
	HazardCount    int        `db:"hazard_count"    json:"hazardCount"`
	IsRecommended  bool       `db:"is_recommended"  json:"isRecommended"`
	Profile        string     `db:"profile"         json:"profile"`
	CreatedAt      time.Time  `db:"created_at"      json:"createdAt"`
	ExpiresAt      time.Time  `db:"expires_at"      json:"expiresAt"`
}

// ─── DTO types used across layers ─────────────────────────────────────────────

type GeoJSONPoint struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

type GeoJSONLineString struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

type GeoJSONPolygon struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Geometry   interface{}            `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

func NewFeatureCollection(features []GeoJSONFeature) GeoJSONFeatureCollection {
	if features == nil {
		features = []GeoJSONFeature{}
	}
	return GeoJSONFeatureCollection{Type: "FeatureCollection", Features: features}
}

// HazardInfo describes a single hazard along a route.
type HazardInfo struct {
	Type        string  `json:"type"`
	Severity    int     `json:"severity"`
	Latitude    float64 `json:"lat"`
	Longitude   float64 `json:"lon"`
	Description string  `json:"description,omitempty"`
}

// RouteResult is the fully scored route returned to the client.
type RouteResult struct {
	ID             int64              `json:"id"`
	IsRecommended  bool               `json:"isRecommended"`
	SafetyScore    float64            `json:"safetyScore"`
	CongestionProb float64            `json:"congestionProb"`
	HazardCount    int                `json:"hazardCount"`
	DistanceM      float64            `json:"distanceM"`
	DurationS      float64            `json:"durationS"`
	ETAMinutes     float64            `json:"etaMinutes"`
	Geometry       GeoJSONLineString  `json:"geometry"`
	Hazards        []HazardInfo       `json:"hazards"`
}

type RouteListResult struct {
	Recommended  RouteResult   `json:"recommended"`
	Alternatives []RouteResult `json:"alternatives"`
}
