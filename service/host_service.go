package service

import (
	"LabPanel/models"
	"net"
	"sort"
	"strings"
)

type HostService struct{}

func NewHostService() *HostService {
	return &HostService{}
}

func (s *HostService) GetHostInfo() (*models.HostInfoResponse, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	lanSet := make(map[string]struct{})
	bridgeSet := make(map[string]struct{})
	recommended := ""

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}

			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}

			ipStr := ip.String()
			if strings.HasPrefix(iface.Name, "lxdbr") {
				bridgeSet[ipStr] = struct{}{}
				if recommended == "" {
					recommended = ipStr
				}
				continue
			}

			if strings.HasPrefix(iface.Name, "docker") || strings.HasPrefix(iface.Name, "br-") || strings.HasPrefix(iface.Name, "virbr") {
				bridgeSet[ipStr] = struct{}{}
				continue
			}

			lanSet[ipStr] = struct{}{}
		}
	}

	lanIPs := mapKeysSorted(lanSet)
	bridgeIPs := mapKeysSorted(bridgeSet)

	if recommended == "" && len(bridgeIPs) > 0 {
		recommended = bridgeIPs[0]
	}

	return &models.HostInfoResponse{
		RecommendedContainerHostIP: recommended,
		LanIPs:                     lanIPs,
		BridgeIPs:                  bridgeIPs,
	}, nil
}

func mapKeysSorted(m map[string]struct{}) []string {
	items := make([]string, 0, len(m))
	for item := range m {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}
