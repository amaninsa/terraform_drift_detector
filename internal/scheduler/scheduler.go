package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/config"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/notify"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/store"
)

// ScanFunc runs a drift scan for a workspace.
type ScanFunc func(ctx context.Context, opts models.ScanOptions) (*models.DriftReport, error)

// Scheduler runs cron-based drift scans.
type Scheduler struct {
	cron    *cron.Cron
	cfg     *config.Config
	store   *store.Store
	scanFn  ScanFunc
	notify  *notify.Client
	webhook []string
	mu      sync.Mutex
	entries map[string]cron.EntryID
}

// New creates a scheduler from configuration.
func New(cfg *config.Config, st *store.Store, scanFn ScanFunc) *Scheduler {
	return &Scheduler{
		cron:    cron.New(),
		cfg:     cfg,
		store:   st,
		scanFn:  scanFn,
		notify:  notify.NewClient(),
		webhook: cfg.ActiveWebhooks(),
		entries: map[string]cron.EntryID{},
	}
}

// Start registers all configured schedules and starts the cron runner.
func (s *Scheduler) Start() error {
	for _, sched := range s.cfg.Schedules {
		if err := s.Add(sched.Workspace, sched.Cron); err != nil {
			return err
		}
	}
	s.cron.Start()
	return nil
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// Add registers a cron schedule for a workspace.
func (s *Scheduler) Add(workspace, expr string) error {
	ws, err := s.cfg.GetWorkspace(workspace)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.entries[workspace]; ok {
		s.cron.Remove(id)
	}
	wsCopy := *ws
	id, err := s.cron.AddFunc(expr, func() {
		s.runScheduled(wsCopy)
	})
	if err != nil {
		return fmt.Errorf("invalid cron %q for workspace %q: %w", expr, workspace, err)
	}
	s.entries[workspace] = id
	log.Printf("scheduled workspace %q with cron %q", workspace, expr)
	return nil
}

func (s *Scheduler) runScheduled(ws config.WorkspaceConfig) {
	ctx := context.Background()
	opts := ws.ToScanOptions(s.cfg.Drift)
	log.Printf("starting scheduled scan for workspace %q", ws.Name)
	report, err := s.scanFn(ctx, opts)
	if err != nil {
		log.Printf("scheduled scan failed for %q: %v", ws.Name, err)
		return
	}
	if s.store != nil {
		if err := s.store.SaveReport(report); err != nil {
			log.Printf("failed to persist scan %s: %v", report.ScanID, err)
		}
	}
	if err := s.notify.NotifyDrift(ctx, s.webhook, report); err != nil {
		log.Printf("webhook notification failed: %v", err)
	}
	log.Printf("scheduled scan %s complete: %d drifted / %d total", report.ScanID, report.Summary.Drifted, report.Summary.TotalResources)
}

// List returns configured schedule expressions.
func (s *Scheduler) List() []config.ScheduleConfig {
	return s.cfg.Schedules
}
