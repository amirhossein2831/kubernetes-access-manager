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
	Env        string           `mapstructure:"env"`
	User       string           `mapstructure:"user"`
	Policy     string           `mapstructure:"policy"`
	Poll       PollConfig       `mapstructure:"poll"`
	Kubernetes KubernetesConfig `mapstructure:"kubernetes"`
	HTTP       HTTPConfig       `mapstructure:"http"`
	Output     OutputConfig     `mapstructure:"output"`
	Log        LogConfig        `mapstructure:"log"`
}

type PollConfig struct {
	Interval time.Duration `mapstructure:"interval"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type KubernetesConfig struct {
	Kubeconfig     string        `mapstructure:"kubeconfig"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	Attempts       int           `mapstructure:"attempts"`
	BackoffMin     time.Duration `mapstructure:"backoff_min"`
	BackoffMax     time.Duration `mapstructure:"backoff_max"`
	QPS            float32       `mapstructure:"qps"`
	Burst          int           `mapstructure:"burst"`
	PageSize       int64         `mapstructure:"page_size"`
}

type HTTPConfig struct {
	Addr            string        `mapstructure:"addr"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type OutputConfig struct {
	Path     string `mapstructure:"path"`
	MaxBytes int64  `mapstructure:"max_bytes"`
	Backups  int    `mapstructure:"backups"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

func Init() (*Config, error) {
	return LoadFrom("configs")
}

func parseEnv() (string, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	if env != "local" && env != "development" && env != "production" {
		return "", errors.New("invalid APP_ENV")
	}

	return env, nil
}

func LoadFrom(dir string) (*Config, error) {
	env, err := parseEnv()
	if err != nil {
		return nil, err
	}

	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetConfigType("yaml")
	v.AddConfigPath(dir)
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetConfigName("config." + env)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}

	var c Config
	if err := v.UnmarshalExact(&c); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}

	return parseAndValidate(c, env)
}

func parseAndValidate(c Config, env string) (*Config, error) {
	c.Env = env
	if c.Poll.Interval <= 0 || c.Poll.Timeout <= 0 || c.Kubernetes.RequestTimeout <= 0 || c.Kubernetes.RequestTimeout > c.Poll.Timeout {
		return nil, errors.New("invalid polling/request duration")
	}
	if c.Kubernetes.Attempts < 1 || c.Kubernetes.Attempts > 10 || c.Kubernetes.BackoffMin <= 0 || c.Kubernetes.BackoffMax < c.Kubernetes.BackoffMin || c.Kubernetes.QPS <= 0 || c.Kubernetes.Burst < 1 || c.Kubernetes.PageSize < 1 {
		return nil, errors.New("invalid Kubernetes request controls")
	}
	if c.HTTP.Addr == "" || c.HTTP.ShutdownTimeout <= 0 || c.Output.Path == "" || c.Output.MaxBytes < 1024 || c.Output.Backups < 1 {
		return nil, errors.New("invalid HTTP/output/input settings")
	}
	return &c, nil
}
