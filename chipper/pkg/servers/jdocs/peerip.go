package jdocsserver

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"google.golang.org/grpc/peer"
)

func peerIPFromContext(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(strings.Split(p.Addr.String(), ":")[0])
}

func shouldUsePeerIP(ipAddr string) bool {
	ipAddr = strings.TrimSpace(ipAddr)
	if ipAddr == "" {
		return false
	}
	gatewayIP := linuxDefaultGatewayIP()
	if gatewayIP != "" && ipAddr == gatewayIP {
		logger.Println("Ignoring peer IP " + ipAddr + " because it is this container's default gateway")
		return false
	}
	return true
}

func linuxDefaultGatewayIP() string {
	routeBytes, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(routeBytes), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		gateway, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			return ""
		}
		return net.IPv4(byte(gateway), byte(gateway>>8), byte(gateway>>16), byte(gateway>>24)).String()
	}
	return ""
}
