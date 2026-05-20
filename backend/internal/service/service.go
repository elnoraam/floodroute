package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/floodroute/backend/internal/auth"
	"github.com/floodroute/backend/internal/config"
	"github.com/floodroute/backend/internal/model"
	"github.com/floodroute/backend/internal/repository"
	"github.com/jmoiron/sqlx"
)

const (
	defaultRouteCenterLat = -6.9175
	defaultRouteCenterLon = 107.6191
	defaultRouteBufferDeg = 0.01
	defaultWeatherTTL     = 30 * time.Minute
)

var (
	ErrValidation   = errors.New("validation error")
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
)

// Service holds the application use cases.
type Service struct {
	Config     *config.Config
	db         *sqlx.DB
	Users      *repository.UserRepository
	Incidents  *repository.IncidentRepository
	FloodZones *repository.FloodZoneRepository
	Weather    *repository.WeatherRepository
	Routes     *repository.RouteRepository
	httpClient *http.Client
	now        func() time.Time
}

// AuthResponse is returned by login and registration endpoints.
type AuthResponse struct {
	Token       string     `json:"token"`
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	DisplayName *string    `json:"displayName,omitempty"`
	Role        model.Role `json:"role"`
	ExpiresAt   time.Time  `json:"expiresAt"`
}

// RegisterInput is the payload accepted by the registration flow.
type RegisterInput struct {
	Username    string
	Email       string
	Password    string
	DisplayName *string
}

// LoginInput is the payload accepted by the login flow.
type LoginInput struct {
	UsernameOrEmail string
	Password        string
}

// IncidentInput represents a user submitted report.
type IncidentInput struct {
	Type        model.IncidentType
	Severity    int16
	Title       *string
	Description *string
	ImageURL    *string
	Latitude    float64
	Longitude   float64
	ExpiresAt   *time.Time
	UserID      *int64
}

// RouteInput is the route request accepted by the API.
type RouteInput struct {
	OriginLat   float64
	OriginLon   float64
	DestLat     float64
	DestLon     float64
	Profile     string
	Alternatives int
}

// New creates a service with repository dependencies wired to the database.
func New(cfg *config.Config, db *sqlx.DB) *Service {
	return &Service{
		Config:     cfg,
		db:         db,
		Users:      repository.NewUserRepository(db),
		Incidents:  repository.NewIncidentRepository(db),
		FloodZones: repository.NewFloodZoneRepository(db),
		Weather:    repository.NewWeatherRepository(db),
		Routes:     repository.NewRouteRepository(db),
		httpClient: &http.Client{Timeout: 20 * time.Second},
		now:        time.Now,
	}
}

// RegisterUser creates a new account and returns the signed token payload.
func (s *Service) RegisterUser(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	username := strings.TrimSpace(input.Username)
	email := strings.TrimSpace(strings.ToLower(input.Email))
	password := input.Password
	if username == "" || email == "" || password == "" {
		return nil, fmt.Errorf("%w: username, email, and password are required", ErrValidation)
	}

	exists, err := s.Users.ExistsByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: username already exists", ErrValidation)
	}

	exists, err = s.Users.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: email already exists", ErrValidation)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		DisplayName:  input.DisplayName,
		Role:         model.RoleUser,
		IsActive:     true,
	}
	if err := s.Users.Create(ctx, user); err != nil {
		return nil, err
	}

	return s.issueAuthResponse(user)
}

// LoginUser validates the supplied credentials and returns a signed token payload.
func (s *Service) LoginUser(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	usernameOrEmail := strings.TrimSpace(input.UsernameOrEmail)
	if usernameOrEmail == "" || strings.TrimSpace(input.Password) == "" {
		return nil, fmt.Errorf("%w: username and password are required", ErrValidation)
	}

	user, err := s.Users.FindByUsername(ctx, usernameOrEmail)
	if err != nil {
		user, err = s.Users.FindByEmail(ctx, strings.ToLower(usernameOrEmail))
		if err != nil {
			return nil, ErrUnauthorized
		}
	}
	if err := auth.ComparePassword(user.PasswordHash, input.Password); err != nil {
		return nil, ErrUnauthorized
	}

	return s.issueAuthResponse(user)
}

// ListIncidents returns active incidents as GeoJSON.
func (s *Service) ListIncidents(ctx context.Context, lat, lon, radius float64) (model.GeoJSONFeatureCollection, error) {
	var list []model.Incident
	var err error
	if lat != 0 || lon != 0 {
		if radius <= 0 {
			radius = 5000
		}
		list, err = s.Incidents.FindNearby(ctx, lat, lon, radius)
	} else {
		list, err = s.Incidents.FindAllActive(ctx)
	}
	if err != nil {
		return model.NewFeatureCollection(nil), err
	}

	features := make([]model.GeoJSONFeature, 0, len(list))
	for _, incident := range list {
		features = append(features, model.GeoJSONFeature{
			Type: "Feature",
			Geometry: model.GeoJSONPoint{
				Type:        "Point",
				Coordinates: [2]float64{incident.Longitude, incident.Latitude},
			},
			Properties: map[string]interface{}{
				"id":          incident.ID,
				"userId":      incident.UserID,
				"type":        incident.Type,
				"severity":    incident.Severity,
				"title":       incident.Title,
				"description": incident.Description,
				"imageUrl":    incident.ImageURL,
				"upvotes":     incident.Upvotes,
				"isVerified":  incident.IsVerified,
				"isActive":    incident.IsActive,
				"reportedAt":  incident.ReportedAt,
				"updatedAt":   incident.UpdatedAt,
			},
		})
	}
	return model.NewFeatureCollection(features), nil
}

// CreateIncident persists a community report.
func (s *Service) CreateIncident(ctx context.Context, input IncidentInput) (*model.Incident, error) {
	if input.Type == "" {
		return nil, fmt.Errorf("%w: incident type is required", ErrValidation)
	}
	if input.Severity < 1 || input.Severity > 5 {
		return nil, fmt.Errorf("%w: severity must be between 1 and 5", ErrValidation)
	}
	if input.Latitude == 0 && input.Longitude == 0 {
		return nil, fmt.Errorf("%w: latitude and longitude are required", ErrValidation)
	}

	expiresAt := s.now().Add(6 * time.Hour)
	if input.ExpiresAt != nil && !input.ExpiresAt.IsZero() {
		expiresAt = *input.ExpiresAt
	}

	incident := &model.Incident{
		UserID:      input.UserID,
		Type:        input.Type,
		Severity:    input.Severity,
		Title:       input.Title,
		Description: input.Description,
		ImageURL:    input.ImageURL,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		ExpiresAt:   expiresAt,
		IsActive:    true,
	}
	if err := s.Incidents.Create(ctx, incident); err != nil {
		return nil, err
	}
	return incident, nil
}

// UpvoteIncident increments the report popularity counter.
func (s *Service) UpvoteIncident(ctx context.Context, id int64) (int, error) {
	if id <= 0 {
		return 0, fmt.Errorf("%w: invalid incident id", ErrValidation)
	}
	return s.Incidents.IncrementUpvotes(ctx, id)
}

// ListFloodZones returns active flood zones as GeoJSON.
func (s *Service) ListFloodZones(ctx context.Context) (model.GeoJSONFeatureCollection, error) {
	zones, err := s.FloodZones.FindAllActive(ctx)
	if err != nil {
		return model.NewFeatureCollection(nil), err
	}

	features := make([]model.GeoJSONFeature, 0, len(zones))
	for _, zone := range zones {
		coords, err := parsePolygonWKT(zone.GeomWKT)
		if err != nil {
			continue
		}
		features = append(features, model.GeoJSONFeature{
			Type: "Feature",
			Geometry: model.GeoJSONPolygon{
				Type:        "Polygon",
				Coordinates: coords,
			},
			Properties: map[string]interface{}{
				"id":            zone.ID,
				"name":          zone.Name,
				"source":        zone.Source,
				"riskLevel":     zone.RiskLevel,
				"rainfallMm":    zone.RainfallMm,
				"incidentCount": zone.IncidentCount,
				"validFrom":     zone.ValidFrom,
				"validUntil":    zone.ValidUntil,
				"createdAt":     zone.CreatedAt,
			},
		})
	}

	return model.NewFeatureCollection(features), nil
}

// GetWeather returns recent weather cache rows and opportunistically refreshes a live sample.
func (s *Service) GetWeather(ctx context.Context) ([]model.WeatherCache, error) {
	rows, err := s.Weather.FindRecent(ctx, s.now().Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	if _, err := s.RefreshWeatherCache(ctx); err == nil {
		return s.Weather.FindRecent(ctx, s.now().Add(-24*time.Hour))
	}
	return rows, nil
}

// CalculateRoutes scores a primary route and a set of alternatives.
func (s *Service) CalculateRoutes(ctx context.Context, input RouteInput, userID *int64) (model.RouteListResult, error) {
	if input.OriginLat == 0 && input.OriginLon == 0 {
		return model.RouteListResult{}, fmt.Errorf("%w: origin coordinates are required", ErrValidation)
	}
	if input.DestLat == 0 && input.DestLon == 0 {
		return model.RouteListResult{}, fmt.Errorf("%w: destination coordinates are required", ErrValidation)
	}
	if input.Profile == "" {
		input.Profile = "driving-car"
	}
	if input.Alternatives < 0 {
		input.Alternatives = 0
	}
	if input.Alternatives > 2 {
		input.Alternatives = 2
	}

	candidates := make([]routeCandidate, 0, 1+input.Alternatives)
	for idx := 0; idx <= input.Alternatives; idx++ {
		coords, err := s.buildCandidateGeometry(ctx, input, idx)
		if err != nil {
			return model.RouteListResult{}, err
		}
		candidate, err := s.scoreCandidate(ctx, input, coords, idx == 0)
		if err != nil {
			return model.RouteListResult{}, err
		}
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Result.SafetyScore == candidates[j].Result.SafetyScore {
			return candidates[i].Result.DistanceM < candidates[j].Result.DistanceM
		}
		return candidates[i].Result.SafetyScore > candidates[j].Result.SafetyScore
	})

	for i := range candidates {
		candidates[i].Result.IsRecommended = i == 0
	}

	for i := range candidates {
		_ = s.cacheRoute(ctx, userID, input, candidates[i])
	}

	alternatives := make([]model.RouteResult, 0, len(candidates)-1)
	for _, candidate := range candidates[1:] {
		alternatives = append(alternatives, candidate.Result)
	}

	return model.RouteListResult{
		Recommended:  candidates[0].Result,
		Alternatives: alternatives,
	}, nil
}

// ExpireIncidents is used by the scheduler.
func (s *Service) ExpireIncidents(ctx context.Context) (int64, error) {
	return s.Incidents.ExpireOld(ctx)
}

// CleanupRoutes removes expired anonymous route cache rows.
func (s *Service) CleanupRoutes(ctx context.Context) (int64, error) {
	return s.Routes.DeleteExpiredAnonymous(ctx)
}

// CleanupWeather removes stale weather cache rows.
func (s *Service) CleanupWeather(ctx context.Context) (int64, error) {
	return s.Weather.DeleteOlderThan(ctx, s.now().Add(-72*time.Hour))
}

// RefreshWeatherCache pulls a live weather snapshot when the API key is available.
func (s *Service) RefreshWeatherCache(ctx context.Context) (*model.WeatherCache, error) {
	if strings.TrimSpace(s.Config.OpenWeatherAPIKey) == "" {
		return nil, errors.New("openweather api key is not configured")
	}
	weather, err := s.fetchOpenWeather(ctx, defaultRouteCenterLat, defaultRouteCenterLon)
	if err != nil {
		return nil, err
	}
	weather.ValidUntil = ptrTime(s.now().Add(defaultWeatherTTL))
	if err := s.Weather.Insert(ctx, weather); err != nil {
		return nil, err
	}
	return weather, nil
}

// RefreshFloodZones is intentionally conservative: it keeps the current active flood zones intact
// and can be extended later to materialize computed zones from upstream rainfall and incident feeds.
func (s *Service) RefreshFloodZones(ctx context.Context) error {
	return nil
}

type routeCandidate struct {
	Result   model.RouteResult
	Coords   [][2]float64
	LineWKT  string
	Hazards  []model.HazardInfo
	Score    float64
	Distance float64
	Duration float64
}

func (s *Service) issueAuthResponse(user *model.User) (*AuthResponse, error) {
	if user == nil {
		return nil, ErrNotFound
	}
	token, err := auth.GenerateToken(user.ID, user.Username, string(user.Role), s.Config.JWTSecret, s.Config.JWTExpirationDur)
	if err != nil {
		return nil, err
	}
	return &AuthResponse{
		Token:       token,
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		ExpiresAt:   s.now().Add(s.Config.JWTExpirationDur),
	}, nil
}

func (s *Service) buildCandidateGeometry(ctx context.Context, input RouteInput, variant int) ([][2]float64, error) {
	if variant == 0 {
		if coords, err := s.fetchOpenRouteService(ctx, input, nil); err == nil && len(coords) >= 2 {
			return coords, nil
		}
	}

	origin := [2]float64{input.OriginLon, input.OriginLat}
	dest := [2]float64{input.DestLon, input.DestLat}
	midLat := (input.OriginLat + input.DestLat) / 2
	midLon := (input.OriginLon + input.DestLon) / 2
	dx := input.DestLon - input.OriginLon
	dy := input.DestLat - input.OriginLat
	span := math.Hypot(dx, dy)
	if span == 0 {
		span = 0.01
	}
	perpLon := -dy / span
	perpLat := dx / span
	offset := span * 0.15
	if offset < 0.001 {
		offset = 0.001
	}
	sign := 0.0
	switch variant {
	case 1:
		sign = 1
	case 2:
		sign = -1
	default:
		sign = 0
	}

	waypoint := [2]float64{midLon + perpLon*offset*sign, midLat + perpLat*offset*sign}
	if coords, err := s.fetchOpenRouteService(ctx, input, &waypoint); err == nil && len(coords) >= 2 {
		return coords, nil
	}
	return [][2]float64{origin, waypoint, dest}, nil
}

func (s *Service) scoreCandidate(ctx context.Context, input RouteInput, coords [][2]float64, primary bool) (routeCandidate, error) {
	lineWKT := coordsToLineWKT(coords)
	incidents, err := s.Incidents.FindAlongRoute(ctx, lineWKT, defaultRouteBufferDeg)
	if err != nil {
		incidents = nil
	}
	zones, err := s.FloodZones.FindIntersectingRoute(ctx, lineWKT)
	if err != nil {
		zones = nil
	}
	weatherRows, err := s.Weather.FindRecent(ctx, s.now().Add(-6*time.Hour))
	if err != nil {
		weatherRows = nil
	}

	distanceM := routeDistanceMeters(coords)
	durationS := routeDurationSeconds(distanceM, input.Profile)
	hazards, hazardPenalty, congestionPenalty := routeHazardsAndPenalties(incidents, zones, weatherRows, coords)
	score := 0.96 - hazardPenalty - congestionPenalty
	if primary {
		score += 0.03
	}
	score = clamp01(score)

	result := model.RouteResult{
		IsRecommended:  primary,
		SafetyScore:    score,
		CongestionProb:  clamp01(1 - score + congestionPenalty*0.6),
		HazardCount:    len(hazards),
		DistanceM:      distanceM,
		DurationS:      durationS,
		ETAMinutes:     durationS / 60,
		Geometry:       model.GeoJSONLineString{Type: "LineString", Coordinates: coords},
		Hazards:        hazards,
	}

	return routeCandidate{
		Result:   result,
		Coords:   coords,
		LineWKT:  lineWKT,
		Hazards:  hazards,
		Score:    score,
		Distance: distanceM,
		Duration: durationS,
	}, nil
}

func (s *Service) cacheRoute(ctx context.Context, userID *int64, input RouteInput, candidate routeCandidate) error {
	if s.db == nil {
		return nil
	}
	meta, _ := json.Marshal(map[string]interface{}{
		"hazards": candidate.Hazards,
		"score":   candidate.Score,
		"profile": input.Profile,
	})
	var userArg any
	if userID != nil {
		userArg = *userID
	}
	const q = `
		INSERT INTO routes
			(user_id, origin_lat, origin_lon, dest_lat, dest_lon,
			 distance_m, duration_s, safety_score, congestion_prob,
			 hazard_count, is_recommended, profile, metadata, expires_at)
		VALUES
			($1, $2, $3, $4, $5,
			 $6, $7, $8, $9,
			 $10, $11, $12, $13, NOW() + INTERVAL '30 minutes')`
	_, err := s.db.ExecContext(ctx, q,
		userArg, input.OriginLat, input.OriginLon, input.DestLat, input.DestLon,
		candidate.Distance, candidate.Duration, candidate.Result.SafetyScore, candidate.Result.CongestionProb,
		candidate.Result.HazardCount, candidate.Result.IsRecommended, input.Profile, bytesOrNil(meta),
	)
	return err
}

func bytesOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

func routeHazardsAndPenalties(incidents []model.Incident, zones []model.FloodZone, weatherRows []model.WeatherCache, coords [][2]float64) ([]model.HazardInfo, float64, float64) {
	hazards := make([]model.HazardInfo, 0, len(incidents)+len(zones))
	incidentPenalty := 0.0
	congestionPenalty := 0.0

	for _, incident := range incidents {
		hazards = append(hazards, model.HazardInfo{
			Type:        string(incident.Type),
			Severity:    int(incident.Severity),
			Latitude:    incident.Latitude,
			Longitude:   incident.Longitude,
			Description: firstNonEmpty(incident.Title, incident.Description),
		})
		incidentPenalty += float64(incident.Severity) * 0.055
		if incident.Type == model.IncidentCongestion {
			congestionPenalty += 0.08
		}
	}

	for _, zone := range zones {
		lat, lon := polygonCentroid(zone.GeomWKT)
		hazards = append(hazards, model.HazardInfo{
			Type:        "FLOOD_ZONE",
			Severity:    int(zone.RiskLevel),
			Latitude:    lat,
			Longitude:   lon,
			Description: strings.TrimSpace(derefString(zone.Name)),
		})
		incidentPenalty += float64(zone.RiskLevel) * 0.075
	}

	if len(weatherRows) > 0 {
		latest := weatherRows[0]
		if latest.Rainfall1h != nil {
			switch {
			case *latest.Rainfall1h > 10:
				incidentPenalty += 0.12
			case *latest.Rainfall1h > 3:
				incidentPenalty += 0.06
			}
		}
	}

	return hazards, incidentPenalty, congestionPenalty
}


func (s *Service) fetchOpenRouteService(ctx context.Context, input RouteInput, viaPoint *[2]float64) ([][2]float64, error) {
	if strings.TrimSpace(s.Config.ORSAPIKey) != "" {
		base := strings.TrimRight(s.Config.ORSBaseURL, "/")
		endpoint := fmt.Sprintf("%s/v2/directions/%s/geojson", base, url.PathEscape(orsProfile(input.Profile)))
		body := map[string]interface{}{
			"coordinates": routeRequestCoordinates(input, viaPoint),
		}
		payload, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", s.Config.ORSAPIKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := s.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.StatusCode >= 300 {
			data, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
			return nil, fmt.Errorf("ors directions failed: %s", strings.TrimSpace(string(data)))
		}

		var decoded struct {
			Features []struct {
				Geometry struct {
					Coordinates [][]float64 `json:"coordinates"`
				} `json:"geometry"`
			} `json:"features"`
		}
		if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
			return nil, err
		}
		if len(decoded.Features) == 0 || len(decoded.Features[0].Geometry.Coordinates) == 0 {
			return nil, errors.New("ors returned no route geometry")
		}

		coords := make([][2]float64, 0, len(decoded.Features[0].Geometry.Coordinates))
		for _, pair := range decoded.Features[0].Geometry.Coordinates {
			if len(pair) < 2 {
				continue
			}
			coords = append(coords, [2]float64{pair[0], pair[1]})
		}
		return coords, nil
	}

	return s.fetchOSRMRoute(ctx, input, viaPoint)
}

func (s *Service) fetchOSRMRoute(ctx context.Context, input RouteInput, viaPoint *[2]float64) ([][2]float64, error) {
	base := "https://router.project-osrm.org"
	coords := routeRequestCoordinates(input, viaPoint)
	if len(coords) < 2 {
		return nil, errors.New("route requires at least origin and destination")
	}
	parts := make([]string, 0, len(coords))
	for _, coord := range coords {
		parts = append(parts, fmt.Sprintf("%f,%f", coord[0], coord[1]))
	}
	endpoint := fmt.Sprintf("%s/route/v1/%s/%s?overview=full&geometries=geojson&alternatives=false&steps=false", base, url.PathEscape(osrmProfile(input.Profile)), strings.Join(parts, ";"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return nil, fmt.Errorf("osrm directions failed: %s", strings.TrimSpace(string(data)))
	}

	var decoded struct {
		Code   string `json:"code"`
		Routes []struct {
			Geometry struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"routes"`
	}
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if !strings.EqualFold(decoded.Code, "Ok") || len(decoded.Routes) == 0 || len(decoded.Routes[0].Geometry.Coordinates) == 0 {
		return nil, errors.New("osrm returned no route geometry")
	}

	result := make([][2]float64, 0, len(decoded.Routes[0].Geometry.Coordinates))
	for _, pair := range decoded.Routes[0].Geometry.Coordinates {
		if len(pair) < 2 {
			continue
		}
		result = append(result, [2]float64{pair[0], pair[1]})
	}
	return result, nil
}

func routeRequestCoordinates(input RouteInput, viaPoint *[2]float64) [][]float64 {
	coords := make([][]float64, 0, 3)
	coords = append(coords, []float64{input.OriginLon, input.OriginLat})
	if viaPoint != nil {
		coords = append(coords, []float64{viaPoint[0], viaPoint[1]})
	}
	coords = append(coords, []float64{input.DestLon, input.DestLat})
	return coords
}

func osrmProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "cycling-regular":
		return "cycling"
	case "foot-walking":
		return "walking"
	default:
		return "driving"
	}
}

func orsProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "cycling-regular":
		return "cycling-regular"
	case "foot-walking":
		return "foot-walking"
	case "driving-hgv":
		return "driving-hgv"
	default:
		return "driving-car"
	}
}

func (s *Service) fetchOpenWeather(ctx context.Context, lat, lon float64) (*model.WeatherCache, error) {
	base := strings.TrimRight(s.Config.OpenWeatherBaseURL, "/")
	endpoint := fmt.Sprintf("%s/weather?lat=%s&lon=%s&appid=%s&units=metric", base, strconv.FormatFloat(lat, 'f', 6, 64), strconv.FormatFloat(lon, 'f', 6, 64), url.QueryEscape(s.Config.OpenWeatherAPIKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return nil, fmt.Errorf("openweather failed: %s", strings.TrimSpace(string(data)))
	}
	rawBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Weather []struct {
			ID          int    `json:"id"`
			Main        string `json:"main"`
			Description string `json:"description"`
		} `json:"weather"`
		Main struct {
			Temp     float64 `json:"temp"`
			Humidity float64 `json:"humidity"`
		} `json:"main"`
		Wind struct {
			Speed float64 `json:"speed"`
			Deg   float64 `json:"deg"`
		} `json:"wind"`
		Rain map[string]float64 `json:"rain"`
	}
	if err := json.Unmarshal(rawBytes, &decoded); err != nil {
		return nil, err
	}
	var conditionCode, conditionText *string
	if len(decoded.Weather) > 0 {
		code := strconv.Itoa(decoded.Weather[0].ID)
		text := decoded.Weather[0].Description
		conditionCode = &code
		conditionText = &text
	}
	r1h := decoded.Rain["1h"]
	rawStr := string(rawBytes)
	weather := &model.WeatherCache{
		Source:        "OPENWEATHER",
		Latitude:      ptrFloat64(lat),
		Longitude:     ptrFloat64(lon),
		Temperature:   ptrFloat64(decoded.Main.Temp),
		Humidity:      ptrFloat64(decoded.Main.Humidity),
		Rainfall1h:    ptrFloat64(r1h),
		WindSpeed:     ptrFloat64(decoded.Wind.Speed),
		WindDirection: ptrFloat64(decoded.Wind.Deg),
		ConditionCode: conditionCode,
		ConditionText: conditionText,
		RawData:       &rawStr,
		ValidUntil:    ptrTime(s.now().Add(defaultWeatherTTL)),
	}
	return weather, nil
}

func ptrFloat64(v float64) *float64 { return &v }

func ptrTime(v time.Time) *time.Time { return &v }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func routeDistanceMeters(coords [][2]float64) float64 {
	if len(coords) < 2 {
		return 0
	}
	distance := 0.0
	for i := 1; i < len(coords); i++ {
		distance += haversine(coords[i-1][1], coords[i-1][0], coords[i][1], coords[i][0])
	}
	return distance
}

func routeDurationSeconds(distanceM float64, profile string) float64 {
	speed := 13.9
	switch strings.ToLower(profile) {
	case "cycling-regular":
		speed = 4.8
	case "foot-walking":
		speed = 1.4
	case "driving-hgv":
		speed = 11.1
	}
	if speed <= 0 {
		speed = 13.9
	}
	return distanceM / speed
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	root := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadius * math.Asin(math.Sqrt(root))
}

func coordsToLineWKT(coords [][2]float64) string {
	parts := make([]string, 0, len(coords))
	for _, coord := range coords {
		parts = append(parts, fmt.Sprintf("%f %f", coord[0], coord[1]))
	}
	return "LINESTRING(" + strings.Join(parts, ", ") + ")"
}

func parsePolygonWKT(wkt string) ([][][2]float64, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(wkt))
	if !strings.HasPrefix(trimmed, "POLYGON") {
		return nil, fmt.Errorf("unsupported geometry: %s", wkt)
	}
	original := strings.TrimSpace(wkt)
	start := strings.Index(original, "((")
	end := strings.LastIndex(original, "))")
	if start < 0 || end < 0 || end <= start+2 {
		return nil, fmt.Errorf("invalid polygon wkt: %s", wkt)
	}
	inner := original[start+2 : end]
	ringText := strings.Split(inner, "),(")[0]
	coords := make([][2]float64, 0)
	for _, pair := range strings.Split(ringText, ",") {
		fields := strings.Fields(strings.TrimSpace(pair))
		if len(fields) < 2 {
			continue
		}
		lon, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return nil, err
		}
		lat, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, err
		}
		coords = append(coords, [2]float64{lon, lat})
	}
	if len(coords) == 0 {
		return nil, fmt.Errorf("empty polygon: %s", wkt)
	}
	return [][][2]float64{coords}, nil
}

func polygonCentroid(wkt string) (float64, float64) {
	rings, err := parsePolygonWKT(wkt)
	if err != nil || len(rings) == 0 || len(rings[0]) == 0 {
		return defaultRouteCenterLat, defaultRouteCenterLon
	}
	ring := rings[0]
	var sumLon, sumLat float64
	for _, coord := range ring {
		sumLon += coord[0]
		sumLat += coord[1]
	}
	count := float64(len(ring))
	return sumLat / count, sumLon / count
}

func firstNonEmpty(title *string, description *string) string {
	if title != nil && strings.TrimSpace(*title) != "" {
		return strings.TrimSpace(*title)
	}
	if description != nil {
		return strings.TrimSpace(*description)
	}
	return ""
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
