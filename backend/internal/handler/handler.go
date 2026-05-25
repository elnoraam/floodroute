package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/floodroute/backend/internal/middleware"
	"github.com/floodroute/backend/internal/model"
	"github.com/floodroute/backend/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

// Handler wires the HTTP layer to the application service.
type Handler struct {
	svc *service.Service
}

// New creates a handler wrapper around the domain service.
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// NewRouter builds the application router.
func NewRouter(svc *service.Service, jwtSecret string) http.Handler {
	h := New(svc)
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.Recoverer, middleware.Logger)
	r.Use(middleware.Authenticate(jwtSecret))

	r.Get("/healthz", h.healthz)
	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/register", h.register)
		r.Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Get("/flood-zones", h.listFloodZones)
			r.Get("/weather", h.listWeather)
			r.Get("/incidents", h.listIncidents)
			r.Get("/media/{id}", h.getMedia)
			r.Post("/routes", h.calculateRoutes)
			r.Post("/incidents/{id}/upvote", h.upvoteIncident)

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole(string(model.RoleProducer), string(model.RoleSuperadmin)))
				r.Post("/incidents", h.createIncident)
				r.Post("/media", h.uploadMedia)
			})

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole(string(model.RoleSuperadmin)))
				r.Get("/admin/users/pending", h.listPendingUsers)
				r.Patch("/admin/users/{id}/approve", h.approveUser)
			})
		})
	})

	return r
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string  `json:"username"`
		Email       string  `json:"email"`
		Password    string  `json:"password"`
		DisplayName *string `json:"displayName"`
		Role        string  `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := h.svc.RegisterUser(r.Context(), service.RegisterInput{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Role:        model.Role(req.Role),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := h.svc.LoginUser(r.Context(), service.LoginInput{
		UsernameOrEmail: req.Username,
		Password:        req.Password,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) listFloodZones(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.ListFloodZones(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *Handler) listWeather(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.GetWeather(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *Handler) listIncidents(w http.ResponseWriter, r *http.Request) {
	lat, _ := parseFloatQuery(r, "lat")
	lon, _ := parseFloatQuery(r, "lon")
	radius, _ := parseFloatQuery(r, "radius")
	data, err := h.svc.ListIncidents(r.Context(), lat, lon, radius)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *Handler) calculateRoutes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OriginLat    float64 `json:"originLat"`
		OriginLon    float64 `json:"originLon"`
		DestLat      float64 `json:"destLat"`
		DestLon      float64 `json:"destLon"`
		Profile      string  `json:"profile"`
		Alternatives int     `json:"alternatives"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var userID *int64
	if claims := middleware.ClaimsFromCtx(r); claims != nil {
		userID = &claims.UserID
	}
	data, err := h.svc.CalculateRoutes(r.Context(), service.RouteInput{
		OriginLat:    req.OriginLat,
		OriginLon:    req.OriginLon,
		DestLat:      req.DestLat,
		DestLon:      req.DestLon,
		Profile:      req.Profile,
		Alternatives: req.Alternatives,
	}, userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *Handler) createIncident(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type        string  `json:"type"`
		Severity    int16   `json:"severity"`
		Title       *string `json:"title"`
		Description *string `json:"description"`
		ImageURL    *string `json:"imageUrl"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		ExpiresAt   *string `json:"expiresAt"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid expiresAt format"))
			return
		}
		expiresAt = &parsed
	}
	var userID *int64
	if claims := middleware.ClaimsFromCtx(r); claims != nil {
		userID = &claims.UserID
	}
	incident, err := h.svc.CreateIncident(r.Context(), service.IncidentInput{
		Type:        model.IncidentType(strings.ToUpper(strings.TrimSpace(req.Type))),
		Severity:    req.Severity,
		Title:       req.Title,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		ExpiresAt:   expiresAt,
		UserID:      userID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, incident)
}

func (h *Handler) upvoteIncident(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("invalid incident id"))
		return
	}
	count, err := h.svc.UpvoteIncident(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"upvotes": count})
}

func (h *Handler) uploadMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
		Base64Data  string `json:"base64Data"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var userID *int64
	if claims := middleware.ClaimsFromCtx(r); claims != nil {
		userID = &claims.UserID
	}
	media, err := h.svc.UploadMedia(r.Context(), service.MediaUploadInput{
		Filename:    req.Filename,
		ContentType: req.ContentType,
		Base64Data:  req.Base64Data,
		UserID:      userID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, media)
}

func (h *Handler) getMedia(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("invalid media id"))
		return
	}
	media, err := h.svc.GetMedia(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", media.ContentType)
	w.Header().Set("Content-Disposition", `inline; filename="`+strings.ReplaceAll(media.Filename, `"`, "")+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(media.Data)
}

func (h *Handler) listPendingUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListPendingUsers(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *Handler) approveUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := h.svc.ApproveUser(r.Context(), id, model.Role(req.Role))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func decodeJSON(r *http.Request, target any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return errors.New("invalid request body")
	}
	return nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrValidation):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, service.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err)
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"message": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseFloatQuery(r *http.Request, key string) (float64, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}
