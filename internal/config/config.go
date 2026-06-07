package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DNS struct {
		Listen string `yaml:"listen"`
	} `yaml:"dns"`
	Web struct {
		Listen string `yaml:"listen"`
	} `yaml:"web"`
	DB struct {
		Path string `yaml:"path"`
	} `yaml:"db"`
	Geo struct {
		Ip2regionV4 string `yaml:"ip2region_v4"`
		Ip2regionV6 string `yaml:"ip2region_v6"`
	} `yaml:"geo"`
	JWT struct {
		Secret      string `yaml:"secret"`
		ExpireHours int    `yaml:"expire_hours"`
	} `yaml:"jwt"`
	Cache struct {
		GeoTtlSec int `yaml:"geo_ttl_sec"`
	} `yaml:"cache"`
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
	Mode string `yaml:"mode"` // "master" 或 "node"

	Controller struct {
		SyncTimeoutSec        int     `yaml:"sync_timeout_sec"`
		HeartbeatIntervalSec  int     `yaml:"heartbeat_interval_sec"`
		HeartbeatTimeoutSec   int     `yaml:"heartbeat_timeout_sec"`
		CheckIntervalSec      int     `yaml:"check_interval_sec"`
		CheckSuccessThreshold float64 `yaml:"check_success_threshold"`
	} `yaml:"controller"`

	Telegram struct {
		Enabled  bool   `yaml:"enabled"`
		BotToken string `yaml:"bot_token"`
		ChatID   string `yaml:"chat_id"`
	} `yaml:"telegram"`

	// 子节点专用配置
	Node struct {
		ID                   int    `yaml:"id"`
		MasterURL            string `yaml:"master_url"`
		Token                string `yaml:"token"`
		APIListen            string `yaml:"api_listen"`
		HeartbeatIntervalSec int    `yaml:"heartbeat_interval_sec"`
	} `yaml:"node"`
    Probe struct {
        IPv4Enabled bool `yaml:"ipv4_enabled"`
        IPv6Enabled bool `yaml:"ipv6_enabled"`
    } `yaml:"probe"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	// 默认值
	if cfg.JWT.ExpireHours == 0 {
		cfg.JWT.ExpireHours = 24
	}
	if cfg.Cache.GeoTtlSec == 0 {
		cfg.Cache.GeoTtlSec = 300
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "error"
	}
	if cfg.Mode == "" {
		cfg.Mode = "master" // 默认主控
	}
	// Controller 默认值
	if cfg.Controller.SyncTimeoutSec == 0 {
		cfg.Controller.SyncTimeoutSec = 10
	}
	if cfg.Controller.HeartbeatIntervalSec == 0 {
		cfg.Controller.HeartbeatIntervalSec = 120
	}
	if cfg.Controller.HeartbeatTimeoutSec == 0 {
		cfg.Controller.HeartbeatTimeoutSec = 360
	}
	if cfg.Controller.CheckIntervalSec == 0 {
		cfg.Controller.CheckIntervalSec = 300
	}
	if cfg.Controller.CheckSuccessThreshold == 0 {
		cfg.Controller.CheckSuccessThreshold = 0.5
	}
	if cfg.Probe.IPv4Enabled == nil {
		cfg.Probe.IPv4Enabled = true
	}
	if cfg.Probe.IPv6Enabled == nil {
  	  cfg.Probe.IPv6Enabled = true
	}
	// 子节点默认心跳间隔
	if cfg.Node.HeartbeatIntervalSec == 0 {
		cfg.Node.HeartbeatIntervalSec = 120
	}
	return cfg, nil
}
