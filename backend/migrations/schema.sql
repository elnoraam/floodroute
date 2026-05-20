-- FloodRoute Database Schema

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    username        VARCHAR(64) UNIQUE NOT NULL,
    email           VARCHAR(128) UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,
    display_name    VARCHAR(128),
    role            VARCHAR(32) NOT NULL DEFAULT 'USER',
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);

-- ============================================================
-- INCIDENTS (community reports)
-- ============================================================
CREATE TABLE IF NOT EXISTS incidents (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    type            VARCHAR(32) NOT NULL CHECK (type IN (
                        'FLOOD','ACCIDENT','CONGESTION','ROAD_CLOSURE','FALLEN_TREE','OTHER'
                    )),
    severity        SMALLINT NOT NULL DEFAULT 2 CHECK (severity BETWEEN 1 AND 5),
    title           VARCHAR(256),
    description     TEXT,
    image_url       TEXT,
    geom            TEXT,
    latitude        DOUBLE PRECISION NOT NULL,
    longitude       DOUBLE PRECISION NOT NULL,
    upvotes         INT NOT NULL DEFAULT 0,
    is_verified     BOOLEAN NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '6 hours'),
    reported_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_incidents_type     ON incidents(type);
CREATE INDEX idx_incidents_active   ON incidents(is_active, expires_at);
CREATE INDEX idx_incidents_reported ON incidents(reported_at DESC);

-- ============================================================
-- FLOOD ZONES (computed + historical)
-- ============================================================
CREATE TABLE IF NOT EXISTS flood_zones (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(256),
    source          VARCHAR(64) NOT NULL DEFAULT 'COMPUTED',  -- COMPUTED | HISTORICAL | BMKG
    risk_level      SMALLINT NOT NULL DEFAULT 2 CHECK (risk_level BETWEEN 1 AND 5),
    geom            TEXT NOT NULL,
    rainfall_mm     DOUBLE PRECISION,
    incident_count  INT NOT NULL DEFAULT 0,
    valid_from      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_flood_zones_risk   ON flood_zones(risk_level);
CREATE INDEX idx_flood_zones_valid  ON flood_zones(valid_from, valid_until);

-- ============================================================
-- WEATHER CACHE (from BMKG + OpenWeather)
-- ============================================================
CREATE TABLE IF NOT EXISTS weather_cache (
    id              BIGSERIAL PRIMARY KEY,
    source          VARCHAR(32) NOT NULL,   -- BMKG | OPENWEATHER
    station_id      VARCHAR(128),
    geom            TEXT,
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    temperature     DOUBLE PRECISION,
    humidity        DOUBLE PRECISION,
    rainfall_1h     DOUBLE PRECISION,
    rainfall_3h     DOUBLE PRECISION,
    rainfall_24h    DOUBLE PRECISION,
    wind_speed      DOUBLE PRECISION,
    wind_direction  DOUBLE PRECISION,
    condition_code  VARCHAR(32),
    condition_text  VARCHAR(256),
    raw_data        JSONB,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until     TIMESTAMPTZ
);

CREATE INDEX idx_weather_fetched    ON weather_cache(fetched_at DESC);
CREATE INDEX idx_weather_source     ON weather_cache(source, fetched_at DESC);

-- ============================================================
-- ROUTES (cached computed routes)
-- ============================================================
CREATE TABLE IF NOT EXISTS routes (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    origin_lat      DOUBLE PRECISION NOT NULL,
    origin_lon      DOUBLE PRECISION NOT NULL,
    dest_lat        DOUBLE PRECISION NOT NULL,
    dest_lon        DOUBLE PRECISION NOT NULL,
    origin_name     VARCHAR(256),
    dest_name       VARCHAR(256),
    geom            TEXT,
    distance_m      DOUBLE PRECISION,
    duration_s      DOUBLE PRECISION,
    safety_score    DOUBLE PRECISION,        -- 0.0 - 1.0 (higher = safer)
    congestion_prob DOUBLE PRECISION,        -- 0.0 - 1.0
    hazard_count    INT NOT NULL DEFAULT 0,
    is_recommended  BOOLEAN NOT NULL DEFAULT FALSE,
    profile         VARCHAR(32) DEFAULT 'driving-car',
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '30 minutes')
);

CREATE INDEX idx_routes_user    ON routes(user_id, created_at DESC);
CREATE INDEX idx_routes_score   ON routes(safety_score DESC);

-- ============================================================
-- HISTORICAL FLOOD DATA
-- ============================================================
CREATE TABLE IF NOT EXISTS historical_floods (
    id              BIGSERIAL PRIMARY KEY,
    location_name   VARCHAR(256),
    geom            TEXT NOT NULL,
    flood_date      DATE NOT NULL,
    depth_cm        DOUBLE PRECISION,
    duration_hours  DOUBLE PRECISION,
    source          VARCHAR(128),
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hist_floods_date ON historical_floods(flood_date DESC);

-- ============================================================
-- CONGESTION HISTORY
-- ============================================================
CREATE TABLE IF NOT EXISTS congestion_history (
    id              BIGSERIAL PRIMARY KEY,
    geom            TEXT NOT NULL,
    hour_of_week    SMALLINT NOT NULL CHECK (hour_of_week BETWEEN 0 AND 167), -- 0=Mon00, 167=Sun23
    congestion_level SMALLINT NOT NULL DEFAULT 2 CHECK (congestion_level BETWEEN 1 AND 5),
    sample_count    INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_congestion_hour ON congestion_history(hour_of_week);

-- Auto-expire old incidents
CREATE OR REPLACE FUNCTION expire_incidents()
RETURNS void LANGUAGE sql AS $$
    UPDATE incidents SET is_active = FALSE
    WHERE is_active = TRUE AND expires_at < NOW();
$$;

-- ============================================================
-- SEED DATA (sample flood zones for Bandung area)
-- ============================================================
INSERT INTO flood_zones (name, source, risk_level, geom, rainfall_mm) VALUES
(
    'Cikapundung Riverside - High Risk',
    'HISTORICAL',
    4,
    'POLYGON((107.605 -6.925, 107.615 -6.925, 107.615 -6.935, 107.605 -6.935, 107.605 -6.925))',
    80.0
),
(
    'Antapani Low Zone',
    'HISTORICAL',
    3,
    'POLYGON((107.655 -6.905, 107.665 -6.905, 107.665 -6.915, 107.655 -6.915, 107.655 -6.905))',
    55.0
),
(
    'Gedebage Industrial Area',
    'HISTORICAL',
    3,
    'POLYGON((107.700 -6.940, 107.720 -6.940, 107.720 -6.955, 107.700 -6.955, 107.700 -6.940))',
    60.0
)
ON CONFLICT DO NOTHING;