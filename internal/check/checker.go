package check

import (
	"net"
	"time"

	"github.com/go-ping/ping"
)

// CheckRecord 对单条记录进行测活，返回 (success, invalid, message)
func CheckRecord(recType, value string) (success bool, invalid bool, message string) {
	switch recType {
	case "A":
		return pingIP(value)
	case "AAAA":
		if !hasIPv6() {
			return false, true, "no IPv6"
		}
		return pingIP(value)
	case "CNAME":
		addrs, err := net.LookupHost(value)
		if err != nil {
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

// pingIP 对 IP 地址执行一次 Ping，超时 3 秒
func pingIP(ip string) (success bool, invalid bool, message string) {
	pinger, err := ping.NewPinger(ip)
	if err != nil {
		return false, false, "failed to create pinger: " + err.Error()
	}
	pinger.Count = 3
	pinger.Timeout = 3 * time.Second
	pinger.SetPrivileged(true)
	err = pinger.Run()
	if err != nil {
		return false, false, "ping failed: " + err.Error()
	}
	stats := pinger.Statistics()
	if stats.PacketsRecv > 0 {
		return true, false, ""
	}
	return false, false, "100% packet loss"
}

// hasIPv6 检查本机是否有 IPv6 地址
func hasIPv6() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if ok && ipnet.IP.To4() == nil && !ipnet.IP.IsLoopback() && ipnet.IP.IsGlobalUnicast() {
				return true
			}
		}
	}
	return false
}
