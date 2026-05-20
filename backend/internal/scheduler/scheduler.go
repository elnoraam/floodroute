package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/floodroute/backend/internal/service"
	"github.com/robfig/cron/v3"
)

// Scheduler manages periodic maintenance jobs.
type Scheduler struct {
	svc  *service.Service
	cron *cron.Cron
}

// New constructs a scheduler for the application service.
func New(svc *service.Service) *Scheduler {
	return &Scheduler{svc: svc}
}

// Start registers the configured jobs and starts the cron runner.
func (s *Scheduler) Start() error {
	if s.cron != nil {
		return nil
	}
	if s.svc == nil || s.svc.Config == nil {
		return fmt.Errorf("scheduler requires service configuration")
	}

	c := cron.New()
	if _, err := c.AddFunc(s.svc.Config.IncidentExpireCron, s.wrapJob(s.runIncidentMaintenance)); err != nil {
		return err
	}
	if _, err := c.AddFunc(s.svc.Config.WeatherFetchCron, s.wrapJob(s.runWeatherRefresh)); err != nil {
		return err
	}
	if _, err := c.AddFunc(s.svc.Config.FloodZoneUpdateCron, s.wrapJob(s.runFloodZoneRefresh)); err != nil {
		return err
	}
	c.Start()
	s.cron = c
	return nil
}

// Stop halts the scheduler and waits for active jobs to drain.
func (s *Scheduler) Stop() context.Context {
	if s.cron == nil {
		return context.Background()
	}
	return s.cron.Stop()
}

func (s *Scheduler) wrapJob(job func(context.Context) error) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = job(ctx)
	}
}

func (s *Scheduler) runIncidentMaintenance(ctx context.Context) error {
	if _, err := s.svc.ExpireIncidents(ctx); err != nil {
		return err
	}
	if _, err := s.svc.CleanupRoutes(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Scheduler) runWeatherRefresh(ctx context.Context) error {
	if _, err := s.svc.RefreshWeatherCache(ctx); err != nil {
		_, _ = s.svc.CleanupWeather(ctx)
		return err
	}
	_, _ = s.svc.CleanupWeather(ctx)
	return nil
}

func (s *Scheduler) runFloodZoneRefresh(ctx context.Context) error {
	return s.svc.RefreshFloodZones(ctx)
}
