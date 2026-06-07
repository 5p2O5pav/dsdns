package main

import (
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dsdns/internal/config"
	"dsdns/internal/db"
	"dsdns/internal/dns"
	"dsdns/internal/geo"
	"dsdns/internal/logger"
	"dsdns/internal/node"
	"dsdns/internal/web"
	"dsdns/internal/worker"
)

func main() {
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger.Init(cfg.Log.Level)

	if cfg.Mode == "node" {
		runNode(cfg)
		return
	}

	// 主控模式
	runMaster(cfg)
}

func runMaster(cfg *config.Config) {
	database, err := db.Open(cfg.DB.Path)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// 初始化主控自身节点
	if err := db.EnsureMasterNode(database); err != nil {
		logger.Error("failed to ensure master node", "error", err)
		os.Exit(1)
	}

	if err := geo.Init(
		cfg.Geo.Ip2regionV4,
		cfg.Geo.Ip2regionV6,
		cfg.Cache.GeoTtlSec,
	); err != nil {
		logger.Error("failed to init geo", "error", err)
		os.Exit(1)
	}
	defer geo.Close()

	// DNS 服务（主控也提供 DNS 解析）
	querier := &db.Querier{DB: database}
	dnsServer := dns.New(cfg.DNS.Listen, querier, geoResolverAdapter{})
	go func() {
		if err := dnsServer.Start(); err != nil {
			logger.Error("dns server error", "error", err)
			os.Exit(1)
		}
	}()

	// Web 管理界面
	webHandler := web.NewHandler(database, cfg)
	go func() {
		if err := webHandler.Start(); err != nil {
			logger.Error("web server error", "error", err)
			os.Exit(1)
		}
	}()

	// 主控自身心跳维护（避免被误判离线）
	go maintainMasterHeartbeat(database, cfg)

	// 启动后台任务（心跳监控、测活调度）
	worker.StartBackgroundTasks(database, cfg)

	logger.Info("dsdns master started successfully")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("shutting down...")
}

func runNode(cfg *config.Config) {
	// 子节点使用独立数据库文件（建议 node.db）
	dbPath := cfg.DB.Path
	if dbPath == "" {
		dbPath = "./node.db"
	}
	database, err := db.OpenNodeDB(dbPath)
	if err != nil {
		logger.Error("failed to open node database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := geo.Init(
		cfg.Geo.Ip2regionV4,
		cfg.Geo.Ip2regionV6,
		cfg.Cache.GeoTtlSec,
	); err != nil {
		logger.Error("failed to init geo", "error", err)
		os.Exit(1)
	}
	defer geo.Close()

	// DNS 服务
	querier := &db.Querier{DB: database}
	dnsServer := dns.New(cfg.DNS.Listen, querier, geoResolverAdapter{})
	go func() {
		if err := dnsServer.Start(); err != nil {
			logger.Error("dns server error", "error", err)
			os.Exit(1)
		}
	}()

	// 子节点内部 API
	nodeAPI := node.NewAPI(database, cfg)
	go func() {
		if err := nodeAPI.Start(); err != nil {
			logger.Error("node api error", "error", err)
			os.Exit(1)
		}
	}()

	// 心跳上报
	go node.StartHeartbeat(cfg)

	logger.Info("dsdns node started successfully")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("node shutting down...")
}

type geoResolverAdapter struct{}

func (g geoResolverAdapter) GetGeoInfo(ip net.IP) (*geo.GeoInfo, int) {
	return geo.GetGeoInfo(ip)
}

func maintainMasterHeartbeat(db *sql.DB, cfg *config.Config) {
	interval := time.Duration(cfg.Controller.HeartbeatIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即更新一次
	updateMasterHeartbeat(db)

	for range ticker.C {
		updateMasterHeartbeat(db)
	}
}

func updateMasterHeartbeat(db *sql.DB) {
	_, err := db.Exec(`UPDATE nodes SET last_heartbeat = CURRENT_TIMESTAMP, status = 'online' WHERE id = 0`)
	if err != nil {
		logger.Error("update master heartbeat failed", "error", err)
	}
}
