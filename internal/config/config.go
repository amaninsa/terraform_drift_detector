package config

import (
	"fmt"
	"os"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"gopkg.in/yaml.v3"
)

// Config is the top-level drift detector configuration.
type Config struct {
	Server     ServerConfig      `yaml:"server"`
	Webhooks   []WebhookConfig   `yaml:"webhooks"`
	Workspaces []WorkspaceConfig `yaml:"workspaces"`
	Schedules  []ScheduleConfig  `yaml:"schedules"`
	Drift      DriftConfig       `yaml:"drift"`
}

// ServerConfig holds API server settings.
type ServerConfig struct {
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
}

// WebhookConfig defines notification endpoints.
type WebhookConfig struct {
	URL     string `yaml:"url"`
	OnDrift bool   `yaml:"on_drift"`
}

// WorkspaceConfig defines a Terraform workspace to scan.
type WorkspaceConfig struct {
	Name            string                 `yaml:"name"`
	State           models.StateSource     `yaml:"state"`
	Provider        string                 `yaml:"provider"`
	Region          string                 `yaml:"region"`
	SubscriptionID  string                 `yaml:"subscription_id"`
	ProjectID       string                 `yaml:"project_id"`
	DetectUnmanaged bool                   `yaml:"detect_unmanaged"`
	Providers       map[string]ProviderCfg `yaml:"providers"`
}

// ProviderCfg holds per-provider overrides inside a workspace.
type ProviderCfg struct {
	Region         string `yaml:"region"`
	SubscriptionID string `yaml:"subscription_id"`
	ProjectID      string `yaml:"project_id"`
}

// ScheduleConfig defines a cron schedule for a workspace.
type ScheduleConfig struct {
	Workspace string `yaml:"workspace"`
	Cron      string `yaml:"cron"`
}

// DriftConfig holds global drift detection settings.
type DriftConfig struct {
	DetectUnmanaged bool     `yaml:"detect_unmanaged"`
	Concurrency     int      `yaml:"concurrency"`
	IgnoreRules     []string `yaml:"ignore_rules"`
}

// Load reads configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.Database == "" {
		c.Server.Database = "./drift.db"
	}
	if c.Drift.Concurrency == 0 {
		c.Drift.Concurrency = 10
	}
	for i := range c.Workspaces {
		ws := &c.Workspaces[i]
		if ws.Provider == "" && len(ws.Providers) == 1 {
			for name := range ws.Providers {
				ws.Provider = name
			}
		}
		if ws.Provider == "" {
			ws.Provider = "aws"
		}
		if p, ok := ws.Providers[ws.Provider]; ok {
			if ws.Region == "" {
				ws.Region = p.Region
			}
			if ws.SubscriptionID == "" {
				ws.SubscriptionID = p.SubscriptionID
			}
			if ws.ProjectID == "" {
				ws.ProjectID = p.ProjectID
			}
		}
		if !ws.DetectUnmanaged {
			ws.DetectUnmanaged = c.Drift.DetectUnmanaged
		}
	}
}

// GetWorkspace returns a workspace by name.
func (c *Config) GetWorkspace(name string) (*WorkspaceConfig, error) {
	for i := range c.Workspaces {
		if c.Workspaces[i].Name == name {
			return &c.Workspaces[i], nil
		}
	}
	return nil, fmt.Errorf("workspace %q not found", name)
}

// ToScanOptions converts a workspace config to scan options.
func (w *WorkspaceConfig) ToScanOptions(global DriftConfig) models.ScanOptions {
	detectUnmanaged := w.DetectUnmanaged
	if !detectUnmanaged {
		detectUnmanaged = global.DetectUnmanaged
	}
	return models.ScanOptions{
		Workspace:       w.Name,
		State:           w.State,
		Provider:        w.Provider,
		Region:          w.Region,
		SubscriptionID:  w.SubscriptionID,
		ProjectID:         w.ProjectID,
		DetectUnmanaged: detectUnmanaged,
		Concurrency:     global.Concurrency,
		IgnoreRules:     global.IgnoreRules,
	}
}

// ActiveWebhooks returns webhook URLs that should fire on drift.
func (c *Config) ActiveWebhooks() []string {
	var urls []string
	for _, wh := range c.Webhooks {
		if wh.OnDrift && wh.URL != "" {
			urls = append(urls, wh.URL)
		}
	}
	return urls
}
