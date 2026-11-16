package models

// PortForward maps a listener on the LabPanel host to a reachable destination.
type PortForward struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	ListenAddress string `json:"listenAddress"`
	ListenPort    int    `json:"listenPort"`
	TargetAddress string `json:"targetAddress"`
	TargetPort    int    `json:"targetPort"`
	Enabled       bool   `json:"enabled"`
	Comment       string `json:"comment"`
}

type PortForwardStatus struct {
	PortForward
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
