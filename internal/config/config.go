package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DB struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"db"`

	API struct {
		Listen string `mapstructure:"listen"`
	} `mapstructure:"api"`

	SSH struct {
		User                 string        `mapstructure:"user"`
		ConnectTimeoutSeconds int           `mapstructure:"connect_timeout_seconds"`
		ConnectTimeout       time.Duration  `mapstructure:"-"`
	} `mapstructure:"ssh"`

	Discovery struct {
		DNS struct {
			Enabled bool `mapstructure:"enabled"`
		} `mapstructure:"dns"`
	} `mapstructure:"discovery"`

	KeyHunt struct {
		Enabled    bool     `mapstructure:"enabled"`
		AllowRoots []string `mapstructure:"allow_roots"`
		MaxFiles   int      `mapstructure:"max_files"`
		MaxDepth   int      `mapstructure:"max_depth"`
	} `mapstructure:"key_hunt"`

	Watcher struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"watcher"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	// Defaults
	v.SetDefault("api.listen", "127.0.0.1:8080")
	v.SetDefault("ssh.user", "root")
	v.SetDefault("ssh.connect_timeout_seconds", 10)
	v.SetDefault("discovery.dns.enabled", true)
	v.SetDefault("key_hunt.enabled", true)
	v.SetDefault("key_hunt.allow_roots", []string{"/home", "/root", "/etc"})
	v.SetDefault("key_hunt.max_files", 20000)
	v.SetDefault("key_hunt.max_depth", 10)
	v.SetDefault("watcher.enabled", false)

	// Env overrides
	v.SetEnvPrefix("KEYSPIDER")
	v.AutomaticEnv()
	_ = v.BindEnv("db.dsn", "KEYSPIDER_DB_DSN")
	_ = v.BindEnv("api.listen", "KEYSPIDER_API_LISTEN")

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	c.SSH.ConnectTimeout = time.Duration(c.SSH.ConnectTimeoutSeconds) * time.Second

	if c.DB.DSN == "" {
		return nil, fmt.Errorf("db.dsn is required (set KEYSPIDER_DB_DSN or config file)")
	}
	return &c, nil
}
