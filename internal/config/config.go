package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Env        string `mapstructure:"env"`
	InputPath  string `mapstructure:"input_path"`
	PolicyPath string `mapstructure:"policy_path"`
	Poll       struct {
		Interval time.Duration `mapstructure:"interval"`
		Timeout  time.Duration `mapstructure:"timeout"`
	} `mapstructure:"poll"`
	Kubernetes struct {
		Kubeconfig     string        `mapstructure:"kubeconfig"`
		RequestTimeout time.Duration `mapstructure:"request_timeout"`
		Attempts       int           `mapstructure:"attempts"`
		BackoffMin     time.Duration `mapstructure:"backoff_min"`
		BackoffMax     time.Duration `mapstructure:"backoff_max"`
		QPS            float32       `mapstructure:"qps"`
		Burst          int           `mapstructure:"burst"`
		PageSize       int64         `mapstructure:"page_size"`
	} `mapstructure:"kubernetes"`
	HTTP struct {
		Addr            string        `mapstructure:"addr"`
		ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	} `mapstructure:"http"`
	Output struct {
		Path     string `mapstructure:"path"`
		MaxBytes int64  `mapstructure:"max_bytes"`
		Backups  int    `mapstructure:"backups"`
	} `mapstructure:"output"`
	Log struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"log"`
}

func LoadFrom(dir string) (*Config, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}
	if env != "local" && env != "staging" && env != "production" {
		return nil, errors.New("invalid APP_ENV")
	}
	v := viper.New()
	v.SetConfigType("yaml")
	v.AddConfigPath(dir)
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	defaults := map[string]any{
		"env": env, "input_path": "defenition/input.json", "policy_path": "configs/sensitivity.json",
		"poll.interval": "5m", "poll.timeout": "2m", "kubernetes.kubeconfig": "/run/secrets/kubeconfig",
		"kubernetes.request_timeout": "20s", "kubernetes.attempts": 3, "kubernetes.backoff_min": "500ms", "kubernetes.backoff_max": "5s",
		"kubernetes.qps": 5, "kubernetes.burst": 10, "kubernetes.page_size": 500,
		"http.addr": ":8080", "http.shutdown_timeout": "10s", "output.path": "./var/log/access-monitor/events.jsonl",
		"output.max_bytes": 20 * 1024 * 1024, "output.backups": 10, "log.level": "info",
	}
	for k, value := range defaults {
		v.SetDefault(k, value)
		if err := v.BindEnv(k); err != nil {
			return nil, err
		}
	}
	for i, name := range []string{"config", "config." + env} {
		v.SetConfigName(name)
		var err error
		if i == 0 {
			err = v.ReadInConfig()
		} else {
			err = v.MergeInConfig()
		}
		var missing viper.ConfigFileNotFoundError
		if err != nil && !errors.As(err, &missing) {
			return nil, fmt.Errorf("read configuration: %w", err)
		}
	}
	var c Config
	if err := v.UnmarshalExact(&c); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}
	if c.Poll.Interval <= 0 || c.Poll.Timeout <= 0 || c.Kubernetes.RequestTimeout <= 0 || c.Kubernetes.RequestTimeout > c.Poll.Timeout {
		return nil, errors.New("invalid polling/request duration")
	}
	if c.Kubernetes.Attempts < 1 || c.Kubernetes.Attempts > 10 || c.Kubernetes.BackoffMin <= 0 || c.Kubernetes.BackoffMax < c.Kubernetes.BackoffMin || c.Kubernetes.QPS <= 0 || c.Kubernetes.Burst < 1 || c.Kubernetes.PageSize < 1 {
		return nil, errors.New("invalid Kubernetes request controls")
	}
	if c.HTTP.Addr == "" || c.HTTP.ShutdownTimeout <= 0 || c.Output.Path == "" || c.Output.MaxBytes < 1024 || c.Output.Backups < 1 || c.InputPath == "" || c.PolicyPath == "" {
		return nil, errors.New("invalid HTTP/output/input settings")
	}
	return &c, nil
}
