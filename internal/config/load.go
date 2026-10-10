package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DefaultConfigPath is the default config file path (kzero-style relative).
const DefaultConfigPath = "kwd.yaml"

// Load reads and validates kwd configuration from YAML at path. An empty path
// falls back to DefaultConfigPath.
func Load(path string) (*Config, error) {
	cfgPath := path
	if strings.TrimSpace(cfgPath) == "" {
		cfgPath = DefaultConfigPath
	}

	v := viper.New()
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("KWD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	bindConfigEnv(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %q: %w", cfgPath, err)
	}

	var raw rawConfig
	if err := v.Unmarshal(&raw); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	cfg := &Config{
		Cluster:           raw.Cluster,
		Kube:              raw.Kube,
		Client:            raw.Client,
		Resources:         raw.Resources,
		Interval:          raw.Interval,
		Retry:             raw.Retry,
		Timeout:           raw.Timeout,
		Color:             raw.Color,
		LogFormat:         raw.LogFormat,
		DryRun:            raw.DryRun,
		Notifications:     raw.Notifications,
		HTTP:              raw.HTTP,
		ConfirmAlert:      raw.ConfirmAlert,
		ConfirmOk:         raw.ConfirmOk,
		RepeatWhileFiring: raw.RepeatWhileFiring,
	}

	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// rawConfig mirrors Config but uses mapstructure tags for viper decoding.
type rawConfig struct {
	Cluster           Cluster        `mapstructure:"cluster"`
	Kube              Kube           `mapstructure:"kube"`
	Client            Client         `mapstructure:"client"`
	Resources         []string       `mapstructure:"resources"`
	Interval          int            `mapstructure:"interval"`
	Retry             Retry          `mapstructure:"retry"`
	Timeout           time.Duration  `mapstructure:"timeout"`
	Color             string         `mapstructure:"color"`
	LogFormat         string         `mapstructure:"log_format"`
	DryRun            bool           `mapstructure:"dry_run"`
	Notifications     *Notifications `mapstructure:"notifications"`
	HTTP              HTTPConfig     `mapstructure:"http"`
	ConfirmAlert      int            `mapstructure:"confirm_alert"`
	ConfirmOk         int            `mapstructure:"confirm_ok"`
	RepeatWhileFiring bool           `mapstructure:"repeat_while_firing"`
}

// bindConfigEnv links nested YAML keys to KWD_* variables so Unmarshal picks
// up overrides. Phase 3: KWD_HTTP_LISTEN, KWD_CONFIRM_ALERT, KWD_CONFIRM_OK,
// KWD_REPEAT_WHILE_FIRING (D-20). http.health_path / metrics_path stay YAML-only.
func bindConfigEnv(v *viper.Viper) {
	for _, key := range []string{
		"cluster.name",
		"cluster.environment",
		"cluster.description",
		"kube.context",
		"client.id",
		"interval",
		"notifications",
		"http.listen",
		"confirm_alert",
		"confirm_ok",
		"repeat_while_firing",
	} {
		_ = v.BindEnv(key)
	}
}

// applyDefaults fills zero values with defaults.
func (c *Config) applyDefaults() error {
	c.Color = strings.TrimSpace(c.Color)
	if c.Color == "" {
		c.Color = "auto"
	}
	c.LogFormat = strings.TrimSpace(c.LogFormat)
	if c.LogFormat == "" {
		c.LogFormat = "text"
	}
	if c.Retry.Attempts == 0 {
		c.Retry.Attempts = 3
	}
	if c.Retry.InitialBackoff == 0 {
		c.Retry.InitialBackoff = time.Second
	}
	if c.Retry.MaxBackoff == 0 {
		c.Retry.MaxBackoff = 8 * time.Second
	}
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
	// SPEC §5 / D-09…D-13: confirm defaults 1; repeat off; HTTP paths when empty.
	if c.ConfirmAlert == 0 {
		c.ConfirmAlert = 1
	}
	if c.ConfirmOk == 0 {
		c.ConfirmOk = 1
	}
	c.HTTP.HealthPath = strings.TrimSpace(c.HTTP.HealthPath)
	if c.HTTP.HealthPath == "" {
		c.HTTP.HealthPath = "/healthz"
	}
	c.HTTP.MetricsPath = strings.TrimSpace(c.HTTP.MetricsPath)
	if c.HTTP.MetricsPath == "" {
		c.HTTP.MetricsPath = "/metrics"
	}
	c.HTTP.Listen = strings.TrimSpace(c.HTTP.Listen)
	return nil
}

// validate enforces the config contract from SPECIFICATIONS.md §5.
func (c *Config) validate() error {
	if c.Resources == nil || len(c.Resources) == 0 {
		return fmt.Errorf("resources must be a non-empty list of kind.namespace/name references")
	}
	for _, ref := range c.Resources {
		if err := validateResourceRef(ref); err != nil {
			return err
		}
	}
	if c.Retry.InitialBackoff > c.Retry.MaxBackoff {
		return fmt.Errorf("retry.initial_backoff (%s) must be <= retry.max_backoff (%s)",
			c.Retry.InitialBackoff, c.Retry.MaxBackoff)
	}
	if c.Interval < 0 {
		return fmt.Errorf("interval must be >= 0 (got %d)", c.Interval)
	}
	if c.ConfirmAlert < 1 {
		return fmt.Errorf("confirm_alert must be >= 1 (got %d)", c.ConfirmAlert)
	}
	if c.ConfirmOk < 1 {
		return fmt.Errorf("confirm_ok must be >= 1 (got %d)", c.ConfirmOk)
	}
	switch c.Color {
	case "auto", "always", "never":
	default:
		return fmt.Errorf("color must be one of auto, always, never (got %q)", c.Color)
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("log_format must be one of text, json (got %q)", c.LogFormat)
	}
	return nil
}
