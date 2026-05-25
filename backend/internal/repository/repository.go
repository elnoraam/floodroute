package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/floodroute/backend/internal/model"
	"github.com/jmoiron/sqlx"
)

// ─── User Repository ──────────────────────────────────────────────────────────

type UserRepository struct{ db *sqlx.DB }

func NewUserRepository(db *sqlx.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	const q = `
		INSERT INTO users (username, email, password_hash, display_name, role, is_active)
		VALUES (:username, :email, :password_hash, :display_name, :role, :is_active)
		RETURNING id, created_at, updated_at`
	rows, err := r.db.NamedQueryContext(ctx, q, u)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE username=$1 AND is_active=true`, username)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) FindByUsernameAnyStatus(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE username=$1`, username)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE email=$1 AND is_active=true`, email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) FindByEmailAnyStatus(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `SELECT * FROM users WHERE email=$1`, email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) FindPending(ctx context.Context) ([]model.User, error) {
	var list []model.User
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, username, email, password_hash, display_name, role, is_active, created_at, updated_at
		FROM users
		WHERE is_active=false
		ORDER BY created_at ASC`)
	return list, err
}

func (r *UserRepository) Approve(ctx context.Context, id int64, role model.Role) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `
		UPDATE users
		SET role=$2, is_active=true, updated_at=NOW()
		WHERE id=$1
		RETURNING id, username, email, password_hash, display_name, role, is_active, created_at, updated_at`, id, role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) ExistsActiveRole(ctx context.Context, role model.Role) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM users WHERE role=$1 AND is_active=true)`, role)
	return exists, err
}

func (r *UserRepository) ActivateByUsername(ctx context.Context, username string, role model.Role) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `
		UPDATE users
		SET role=$2, is_active=true, updated_at=NOW()
		WHERE username=$1
		RETURNING id, username, email, password_hash, display_name, role, is_active, created_at, updated_at`, username, role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM users WHERE username=$1)`, username)
	return exists, err
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)`, email)
	return exists, err
}

// ─── Incident Repository ──────────────────────────────────────────────────────

type IncidentRepository struct{ db *sqlx.DB }

func NewIncidentRepository(db *sqlx.DB) *IncidentRepository { return &IncidentRepository{db: db} }

func (r *IncidentRepository) Create(ctx context.Context, inc *model.Incident) error {
	const q = `
		INSERT INTO incidents
			(user_id, type, severity, title, description, image_url, latitude, longitude, expires_at)
		VALUES
			($1, $2, $3, $4, $5, $6,
			 $7, $8,
			 $9)
		RETURNING id, reported_at, updated_at`
	return r.db.QueryRowContext(ctx, q,
		inc.UserID, inc.Type, inc.Severity, inc.Title, inc.Description, inc.ImageURL,
		inc.Latitude, inc.Longitude, inc.ExpiresAt,
	).Scan(&inc.ID, &inc.ReportedAt, &inc.UpdatedAt)
}

func (r *IncidentRepository) FindAllActive(ctx context.Context) ([]model.Incident, error) {
	var list []model.Incident
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, user_id, type, severity, title, description, image_url,
		       latitude, longitude, upvotes, is_verified, is_active, expires_at,
		       reported_at, updated_at
		FROM incidents
		WHERE is_active=true AND expires_at > NOW()
		ORDER BY reported_at DESC`)
	return list, err
}

func (r *IncidentRepository) FindNearby(ctx context.Context, lat, lon, radiusM float64) ([]model.Incident, error) {
	var list []model.Incident
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, user_id, type, severity, title, description, image_url,
		       latitude, longitude, upvotes, is_verified, is_active, expires_at,
		       reported_at, updated_at
		FROM incidents
		WHERE is_active=true AND expires_at > NOW()
		  AND latitude BETWEEN $1 - ($3 / 111320.0) AND $1 + ($3 / 111320.0)
		  AND longitude BETWEEN $2 - ($3 / (111320.0 * GREATEST(COS(RADIANS($1)), 0.000001)))
		                  AND $2 + ($3 / (111320.0 * GREATEST(COS(RADIANS($1)), 0.000001)))
		ORDER BY ((latitude - $1) * (latitude - $1)) + ((longitude - $2) * (longitude - $2))`,
		lat, lon, radiusM)
	return list, err
}

// FindAlongRoute returns incidents within bufferDeg degrees of a WKT LineString.
func (r *IncidentRepository) FindAlongRoute(ctx context.Context, lineWKT string, bufferDeg float64) ([]model.Incident, error) {
	var list []model.Incident
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, user_id, type, severity, title, description, image_url,
		       latitude, longitude, upvotes, is_verified, is_active, expires_at,
		       reported_at, updated_at
		FROM incidents
		WHERE is_active=true AND expires_at > NOW()`)
	return list, err
}

func (r *IncidentRepository) IncrementUpvotes(ctx context.Context, id int64) (int, error) {
	var upvotes int
	err := r.db.QueryRowContext(ctx,
		`UPDATE incidents SET upvotes=upvotes+1 WHERE id=$1 RETURNING upvotes`, id,
	).Scan(&upvotes)
	return upvotes, err
}

func (r *IncidentRepository) ExpireOld(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE incidents SET is_active=false WHERE is_active=true AND expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ─── FloodZone Repository ─────────────────────────────────────────────────────

type FloodZoneRepository struct{ db *sqlx.DB }

func NewFloodZoneRepository(db *sqlx.DB) *FloodZoneRepository { return &FloodZoneRepository{db: db} }

func (r *FloodZoneRepository) FindAllActive(ctx context.Context) ([]model.FloodZone, error) {
	var list []model.FloodZone
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, name, source, risk_level, rainfall_mm, incident_count,
		       valid_from, valid_until, created_at,
		       geom AS geom_wkt
		FROM flood_zones
		WHERE valid_until IS NULL OR valid_until > NOW()
		ORDER BY risk_level DESC`)
	return list, err
}

// FindIntersectingRoute returns flood zones that intersect the given WKT LineString.
func (r *FloodZoneRepository) FindIntersectingRoute(ctx context.Context, lineWKT string) ([]model.FloodZone, error) {
	var list []model.FloodZone
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, name, source, risk_level, rainfall_mm, incident_count,
		       valid_from, valid_until, created_at,
		       geom AS geom_wkt
		FROM flood_zones
		WHERE (valid_until IS NULL OR valid_until > NOW())
		ORDER BY risk_level DESC`)
	return list, err
}

// ─── WeatherCache Repository ──────────────────────────────────────────────────

type WeatherRepository struct{ db *sqlx.DB }

func NewWeatherRepository(db *sqlx.DB) *WeatherRepository { return &WeatherRepository{db: db} }

func (r *WeatherRepository) Insert(ctx context.Context, w *model.WeatherCache) error {
	const q = `
		INSERT INTO weather_cache
			(source, station_id, latitude, longitude,
			 temperature, humidity, rainfall_1h, rainfall_3h, rainfall_24h,
			 wind_speed, wind_direction, condition_code, condition_text, raw_data, valid_until)
		VALUES
			($1, $2, $3, $4,
			 $5, $6, $7, $8, $9,
			 $10, $11, $12, $13, $14, $15)
		RETURNING id, fetched_at`
	return r.db.QueryRowContext(ctx, q,
		w.Source, w.StationID,
		w.Latitude, w.Longitude,
		w.Temperature, w.Humidity, w.Rainfall1h, w.Rainfall3h, w.Rainfall24h,
		w.WindSpeed, w.WindDirection, w.ConditionCode, w.ConditionText,
		w.RawData, w.ValidUntil,
	).Scan(&w.ID, &w.FetchedAt)
}

func (r *WeatherRepository) FindRecent(ctx context.Context, since time.Time) ([]model.WeatherCache, error) {
	var list []model.WeatherCache
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, source, station_id, latitude, longitude,
		       temperature, humidity, rainfall_1h, rainfall_3h, rainfall_24h,
		       wind_speed, wind_direction, condition_code, condition_text,
		       fetched_at, valid_until
		FROM weather_cache
		WHERE fetched_at > $1
		ORDER BY fetched_at DESC`, since)
	return list, err
}

func (r *WeatherRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM weather_cache WHERE fetched_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ─── Route Repository ─────────────────────────────────────────────────────────

type RouteRepository struct{ db *sqlx.DB }

func NewRouteRepository(db *sqlx.DB) *RouteRepository { return &RouteRepository{db: db} }

func (r *RouteRepository) FindCached(ctx context.Context, oLat, oLon, dLat, dLon float64) ([]model.Route, error) {
	const tol = 0.001
	var list []model.Route
	err := r.db.SelectContext(ctx, &list, `
		SELECT id, user_id, origin_lat, origin_lon, dest_lat, dest_lon,
		       origin_name, dest_name, distance_m, duration_s,
		       safety_score, congestion_prob, hazard_count,
		       is_recommended, profile, created_at, expires_at
		FROM routes
		WHERE origin_lat BETWEEN $1-$5 AND $1+$5
		  AND origin_lon BETWEEN $2-$5 AND $2+$5
		  AND dest_lat   BETWEEN $3-$5 AND $3+$5
		  AND dest_lon   BETWEEN $4-$5 AND $4+$5
		  AND expires_at > NOW()
		ORDER BY safety_score DESC`,
		oLat, oLon, dLat, dLon, tol)
	return list, err
}

func (r *RouteRepository) DeleteExpiredAnonymous(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM routes WHERE expires_at < NOW() AND user_id IS NULL`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
