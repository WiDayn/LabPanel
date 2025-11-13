package models

type Proxy struct {
	Name      string `json:"name" toml:"name"`
	Type      string `json:"type" toml:"type"`
	LocalIP   string `json:"localIP" toml:"localIP"`
	LocalPort int    `json:"localPort" toml:"localPort"`
	RemotePort int   `json:"remotePort" toml:"remotePort"`
	Comment   string `json:"comment" toml:"-"` // comment不写入toml，存储在单独文件
}

// ProxyComments 存储代理备注的映射
type ProxyComments map[string]string

// ProxyForToml 用于序列化到toml的代理结构（不包含comment）
type ProxyForToml struct {
	Name      string `toml:"name"`
	Type      string `toml:"type"`
	LocalIP   string `toml:"localIP"`
	LocalPort int    `toml:"localPort"`
	RemotePort int   `toml:"remotePort"`
}

type AuthConfig struct {
	Method string `toml:"method"`
	Token  string `toml:"token"`
}

type WebServerConfig struct {
	Addr string `toml:"addr"`
	Port int    `toml:"port"`
}

type FrpConfig struct {
	ServerAddr string          `toml:"serverAddr"`
	ServerPort int             `toml:"serverPort"`
	Auth       AuthConfig      `toml:"auth"`
	WebServer  WebServerConfig `toml:"webServer"`
	Proxies    []Proxy         `toml:"proxies"`
}

type ProxyListResponse struct {
	Config  FrpConfig `json:"config"`
	Proxies []Proxy   `json:"proxies"`
}

type AddProxyRequest struct {
	Proxy Proxy `json:"proxy" binding:"required"`
}

type UpdateProxyRequest struct {
	Index *int  `json:"index" binding:"required"`
	Proxy Proxy `json:"proxy" binding:"required"`
}

type DeleteProxyRequest struct {
	Index int `json:"index" binding:"required"`
}

