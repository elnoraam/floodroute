package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	ServerPort string

	// Database
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string

	// JWT
	JWTSecret        string
	JWTExpirationDur time.Duration

	// Bootstrap account
	SuperadminUsername string
	SuperadminEmail    string
	SuperadminPassword string

	// External APIs
	OpenWeatherAPIKey  string
	OpenWeatherBaseURL string
	ORSAPIKey          string
	ORSBaseURL         string
	BMKGForecastURL    string

	// Rate limiting
	RouteRequestsPerMinute  int
	ReportSubmissionsPerHour int

	// Schedules (cron expressions)
	WeatherFetchCron   string
	FloodZoneUpdateCron string
	IncidentExpireCron string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		ServerPort:              getEnv("SERVER_PORT", "8080"),
		DBHost:                  getEnv("DB_HOST", "localhost"),
		DBPort:                  getEnv("DB_PORT", "5432"),
		DBName:                  getEnv("DB_NAME", "floodroute"),
		DBUser:                  getEnv("DB_USER", "postgres"),
		DBPassword:              getEnv("DB_PASS", "postgres"),
		DBSSLMode:               getEnv("DB_SSLMODE", "disable"),
		JWTSecret:               getEnv("JWT_SECRET", "floodroute-change-me-in-production-256bit"),
		SuperadminUsername:      getEnv("SUPERADMIN_USERNAME", "superadmin"),
		SuperadminEmail:         getEnv("SUPERADMIN_EMAIL", "superadmin@floodroute.local"),
		SuperadminPassword:      getEnv("SUPERADMIN_PASSWORD", "superadmin123"),
		OpenWeatherAPIKey:       getEnv("OPENWEATHER_API_KEY", ""),
		OpenWeatherBaseURL:      getEnv("OPENWEATHER_BASE_URL", "https://api.openweathermap.org/data/2.5"),
		ORSAPIKey:               getEnv("ORS_API_KEY", ""),
		ORSBaseURL:              getEnv("ORS_BASE_URL", "https://api.openrouteservice.org"),
		BMKGForecastURL:         getEnv("BMKG_FORECAST_URL", "https://api.bmkg.go.id/publik/prakiraan-cuaca"),
		WeatherFetchCron:        getEnv("WEATHER_FETCH_CRON", "*/15 * * * *"),
		FloodZoneUpdateCron:     getEnv("FLOOD_ZONE_UPDATE_CRON", "0 * * * *"),
		IncidentExpireCron:      getEnv("INCIDENT_EXPIRE_CRON", "*/5 * * * *"),
	}

	jwtExpHours, err := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "24"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRATION_HOURS: %w", err)
	}
	cfg.JWTExpirationDur = time.Duration(jwtExpHours) * time.Hour

	cfg.RouteRequestsPerMinute, _ = strconv.Atoi(getEnv("RATE_ROUTE_PER_MIN", "20"))
	cfg.ReportSubmissionsPerHour, _ = strconv.Atoi(getEnv("RATE_REPORT_PER_HOUR", "10"))

	if cfg.JWTSecret == "floodroute-change-me-in-production-256bit" {
		fmt.Println("[WARN] Using default JWT secret — set JWT_SECRET in production")
	}

	return cfg, nil
}

// DSN returns the PostgreSQL data source name.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBName, c.DBUser, c.DBPassword, c.DBSSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
