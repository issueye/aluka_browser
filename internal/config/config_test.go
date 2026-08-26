package config

import (
	"testing"
)

func TestBuildProxyArgs(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "禁用代理",
			cfg: Config{
				ProxyEnabled: false,
				ProxyType:    "http",
				ProxyServer:  "127.0.0.1:7890",
			},
			want: "",
		},
		{
			name: "启用 HTTP 代理（无 scheme）",
			cfg: Config{
				ProxyEnabled: true,
				ProxyType:    "http",
				ProxyServer:  "127.0.0.1:7890",
				ProxyBypass:  "<local>;localhost;127.0.0.1",
			},
			want: "--proxy-server=http://127.0.0.1:7890 --proxy-bypass-list=<local>;localhost;127.0.0.1",
		},
		{
			name: "启用 SOCKS5 代理",
			cfg: Config{
				ProxyEnabled: true,
				ProxyType:    "socks5",
				ProxyServer:  "127.0.0.1:10808",
				ProxyBypass:  "localhost",
			},
			want: "--proxy-server=socks5://127.0.0.1:10808 --proxy-bypass-list=localhost",
		},
		{
			name: "代理地址为空",
			cfg: Config{
				ProxyEnabled: true,
				ProxyType:    "http",
				ProxyServer:  "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildProxyArgs(tt.cfg); got != tt.want {
				t.Errorf("BuildProxyArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}
