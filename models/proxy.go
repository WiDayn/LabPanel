package models

type Proxy struct {
	Name      string `json:"name" toml:"name"`
	Type      string `json:"type" toml:"type"`
	LocalIP   string `json:"localIP" toml:"localIP"`
	LocalPort int    `json:"localPort" toml:"localPort"`
	RemotePort int   `json:"remotePort" toml:"remotePort"`
}

type AuthConfig struct {
	Method string `toml:"method"`
	Token  string `toml:"token"`
}

type FrpConfig struct {
	ServerAddr string     `toml:"serverAddr"`
	ServerPort int        `toml:"serverPort"`
	Auth       AuthConfig `toml:"auth"`
	Proxies    []Proxy    `toml:"proxies"`
}

type ProxyListResponse struct {
	Config  FrpConfig `json:"config"`
	Proxies []Proxy   `json:"proxies"`
}

type AddProxyRequest struct {
	Proxy Proxy `json:"proxy" binding:"required"`
}

type UpdateProxyRequest struct {
	Index int   `json:"index" binding:"required"`
	Proxy Proxy `json:"proxy" binding:"required"`
}

type DeleteProxyRequest struct {
	Index int `json:"index" binding:"required"`
}

