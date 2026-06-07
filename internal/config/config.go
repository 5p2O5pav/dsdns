package config

import (
	"fmt"
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

	// 测活能力配置（使用指针 bool 以便区分“未配置”和“配置为 false”）
	// 必须显式配置，程序不提供默认值
	Probe struct {
		IPv4Enabled *bool `yaml:"ipv4_enabled"`
		IPv6Enabled *bool `yaml:"ipv6_enabled"`
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

	// 以下配置项提供默认值（非强制）
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
		cfg.Mode = "master"
	}
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
	if cfg.Node.HeartbeatIntervalSec == 0 {
		cfg.Node.HeartbeatIntervalSec = 120
	}

	// 强制要求 probe 配置，不提供默认值
	// 使用指针 *bool 是因为 bool 的零值是 false，无法区分“用户设为 false”和“用户未配置”
	// 用户必须在配置文件中显式写入 ipv4_enabled 和 ipv6_enabled 字段
	if cfg.Probe.IPv4Enabled == nil {
		return nil, fmt.Errorf("missing required config: probe.ipv4_enabled (must be true or false)")
	}
	if cfg.Probe.IPv6Enabled == nil {
		return nil, fmt.Errorf("missing required config: probe.ipv6_enabled (must be true or false)")
	}

	return cfg, nil
}
