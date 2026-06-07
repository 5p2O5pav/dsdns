package check

import (
    "net"
    "time"
    "context"

    "github.com/go-ping/ping"
    "dsdns/internal/logger"
)

var (
    IPv4ProbeEnabled bool
    IPv6ProbeEnabled bool
)

func SetProbeFlags(ipv4, ipv6 bool) {
    IPv4ProbeEnabled = ipv4
    IPv6ProbeEnabled = ipv6
}

func CheckRecord(recType, value string) (success bool, invalid bool, message string) {
    logger.Debug("CheckRecord start", "type", recType, "value", value)
    switch recType {
    case "A":
        if !IPv4ProbeEnabled {
            return false, true, "IPv4 probe disabled by config"
        }
        return pingIP(value)
    case "AAAA":
        if !IPv6ProbeEnabled {
            return false, true, "IPv6 probe disabled by config"
        }
        return pingIP(value)
    case "CNAME":
        ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
        defer cancel()
        var addrs []string
        var err error
        done := make(chan struct{})
        go func() {
            addrs, err = net.LookupHost(value)
            close(done)
        }()
        select {
        case <-done:
            if err != nil {
                return false, false, "CNAME resolution failed: " + err.Error()
            }
            if len(addrs) == 0 {
                return false, false, "no resolved IP"
            }
            // 对于 CNAME 解析出的 IP，不检查协议开关，直接 ping（pingIP 内部会尝试）
            return pingIP(addrs[0])
        case <-ctx.Done():
            return false, false, "CNAME resolution timeout"
        }
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
