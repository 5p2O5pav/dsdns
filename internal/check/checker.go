// checker.go 修改部分

import (
    "time"
    "fmt"
    "github.com/go-ping/ping"
    "dsdns/internal/logger"   // 确保引入项目 logger
)

func CheckRecord(recType, value string) (success bool, invalid bool, message string) {
    logger.Debug("CheckRecord start", "type", recType, "value", value)
    switch recType {
    case "A":
        return pingIP(value)
    case "AAAA":
        if !hasIPv6() {
            logger.Warn("Skip AAAA check: no IPv6 available", "value", value)
            return false, true, "no IPv6"
        }
        return pingIP(value)
    case "CNAME":
        addrs, err := net.LookupHost(value)
        if err != nil {
            logger.Warn("CNAME resolution failed", "value", value, "error", err)
            return false, false, "CNAME resolution failed: " + err.Error()
        }
        if len(addrs) == 0 {
            return false, false, "no resolved IP"
        }
        return pingIP(addrs[0])
    default:
        return false, false, "unsupported record type"
    }
}

func pingIP(ip string) (success bool, invalid bool, message string) {
    logger.Debug("Starting ping", "ip", ip)
    pinger, err := ping.NewPinger(ip)
    if err != nil {
        logger.Error("Failed to create pinger", "ip", ip, "error", err)
        return false, false, "failed to create pinger: " + err.Error()
    }
    pinger.Count = 3
    pinger.Timeout = 3 * time.Second
    pinger.SetPrivileged(true)

    // 增加完成通道，避免 Run 阻塞过久
    done := make(chan struct{})
    go func() {
        err = pinger.Run()
        close(done)
    }()

    select {
    case <-done:
        if err != nil {
            logger.Error("Ping run error", "ip", ip, "error", err)
            return false, false, "ping run error: " + err.Error()
        }
        stats := pinger.Statistics()
        logger.Debug("Ping finished", "ip", ip, "sent", stats.PacketsSent, "recv", stats.PacketsRecv)
        if stats.PacketsRecv > 0 {
            return true, false, ""
        }
        return false, false, "100% packet loss"
    case <-time.After(5 * time.Second):
        logger.Error("Ping timeout (5s)", "ip", ip)
        return false, false, "ping timeout"
    }
}
