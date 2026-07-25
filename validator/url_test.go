package validator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "empty url is rejected", url: "", wantErr: true},
		{name: "blank url is rejected", url: "   ", wantErr: true},
		{name: "url over 2048 chars is rejected", url: "https://example.com/" + strings.Repeat("a", 2048), wantErr: true},
		{name: "malformed url is rejected", url: "http://exa mple.com", wantErr: true},
		{name: "ftp scheme is rejected", url: "ftp://example.com", wantErr: true},
		{name: "file scheme is rejected", url: "file:///etc/passwd", wantErr: true},
		{name: "javascript scheme is rejected", url: "javascript:alert(1)", wantErr: true},
		{name: "url without a host is rejected", url: "http://", wantErr: true},
		{name: "localhost is rejected", url: "http://localhost", wantErr: true},
		{name: "localhost is rejected case-insensitively", url: "http://LOCALHOST:8080", wantErr: true},
		{name: "loopback IPv4 is rejected", url: "http://127.0.0.1", wantErr: true},
		{name: "loopback IPv6 is rejected", url: "http://[::1]", wantErr: true},
		{name: "private network 10.x is rejected", url: "http://10.0.0.5", wantErr: true},
		{name: "private network 172.16.x is rejected", url: "http://172.16.0.5", wantErr: true},
		{name: "private network 192.168.x is rejected", url: "http://192.168.1.5", wantErr: true},
		{name: "link-local address is rejected", url: "http://169.254.1.1", wantErr: true},
		{name: "unspecified address is rejected", url: "http://0.0.0.0", wantErr: true},
		{name: "valid http url is accepted", url: "http://example.com", wantErr: false},
		{name: "valid https url is accepted", url: "https://example.com", wantErr: false},
		{name: "valid url with path and port is accepted", url: "https://example.com:8443/path?query=1", wantErr: false},
		{name: "public IP is accepted", url: "https://8.8.8.8", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
