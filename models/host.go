package models

type HostInfoResponse struct {
	RecommendedContainerHostIP string   `json:"recommendedContainerHostIP"`
	LanIPs                     []string `json:"lanIPs"`
	BridgeIPs                  []string `json:"bridgeIPs"`
}
