package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

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

func parseLatLon(s string) (float64, float64, error) {
	// Accept formats: "lat,lon" or "lat, lon"
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid coord format")
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, err
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, err
	}
	return lat, lon, nil
}

func geocodePlace(q string) (float64, float64, error) {
	// Fallback geocoding using Nominatim (OSM). Returns lat, lon.
	url := "https://nominatim.openstreetmap.org/search?format=json&limit=1&q=" + strings.ReplaceAll(q, " ", "+")
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "FloodRoute/1.0 (+https://example.com)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var results []map[string]interface{}
	if err := json.Unmarshal(body, &results); err != nil {
		return 0, 0, err
	}
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("no geocoding result")
	}
	latStr, _ := results[0]["lat"].(string)
	lonStr, _ := results[0]["lon"].(string)
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return 0, 0, err
	}
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		return 0, 0, err
	}
	return lat, lon, nil
}

func (s *routingService) GetSafeRoute(origin, destination string) (*models.RouteResponse, error) {
	var oLat, oLon, dLat, dLon float64
	var err error

	oLat, oLon, err = parseLatLon(origin)
	if err != nil {
		// try geocoding
		oLat, oLon, err = geocodePlace(origin)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve origin: %v", err)
		}
	}

	dLat, dLon, err = parseLatLon(destination)
	if err != nil {
		dLat, dLon, err = geocodePlace(destination)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve destination: %v", err)
		}
	}

	// Build ORS request
	orsKey := os.Getenv("ORS_API_KEY")
	if orsKey == "" {
		return nil, fmt.Errorf("ORS_API_KEY not set")
	}

	reqBody := map[string]interface{}{
		"coordinates": [][]float64{{oLon, oLat}, {dLon, dLat}},
		"geometry_format": "geojson",
	}
	jb, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://api.openrouteservice.org/v2/directions/driving-car", bytes.NewReader(jb))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", orsKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ORS request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ORS error: %s", string(body))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse ORS response: %v", err)
	}

	var geometryObj interface{}
	var distance float64
	var duration float64

	// ORS can return a GeoJSON FeatureCollection or routes. Try both.
	if features, ok := parsed["features"].([]interface{}); ok && len(features) > 0 {
		feat0 := features[0].(map[string]interface{})
		geometryObj = feat0["geometry"]
		if props, ok := feat0["properties"].(map[string]interface{}); ok {
			if summary, ok := props["summary"].(map[string]interface{}); ok {
				if d, ok := summary["distance"].(float64); ok {
					distance = d
				}
				if du, ok := summary["duration"].(float64); ok {
					duration = du
				}
			}
		}
	} else if routes, ok := parsed["routes"].([]interface{}); ok && len(routes) > 0 {
		r0 := routes[0].(map[string]interface{})
		if geom, ok := r0["geometry"]; ok {
			geometryObj = geom
		}
		if summary, ok := r0["summary"].(map[string]interface{}); ok {
			if d, ok := summary["distance"].(float64); ok {
				distance = d
			}
			if du, ok := summary["duration"].(float64); ok {
				duration = du
			}
		}
	}

	if geometryObj == nil {
		return nil, fmt.Errorf("no geometry found in ORS response")
	}

	geoBytes, _ := json.Marshal(geometryObj)
	geoJSON := string(geoBytes)

	routeResp := &models.RouteResponse{
		Distance: distance,
		Duration: duration,
		Geometry: geoJSON,
	}

	// Analyze hazards along the route using PostGIS
	hazards, err := s.incidentRepo.GetByRouteGeometry(geoJSON, 100) // 100m buffer
	if err != nil {
		return nil, fmt.Errorf("failed to analyze hazards: %v", err)
	}
	routeResp.Hazards = hazards

	// Compute simple Risk Score
	if len(hazards) > 0 {
		var totalSeverity int
		for _, h := range hazards {
			totalSeverity += h.Severity
		}
		routeResp.RiskScore = float64(totalSeverity) / (float64(len(hazards)) * 5.0)
	} else {
		routeResp.RiskScore = 0
	}

	return routeResp, nil
}
