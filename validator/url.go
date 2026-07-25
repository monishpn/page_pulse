package validator

import (
	"net"
	"net/url"
	"strings"

	gofrHttp "gofr.dev/pkg/gofr/http"
)

const maxURLLength = 2048

// ValidateURL enforces FR-2: the url must be non-empty, at most 2048
// characters, http/https only, and must not target localhost or a
// private network.
func ValidateURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return gofrHttp.ErrorInvalidParam{Params: []string{"empty url"}}
	}

	if len(raw) > maxURLLength {
		return gofrHttp.ErrorInvalidParam{Params: []string{"url too long"}}
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return gofrHttp.ErrorInvalidParam{Params: []string{"url is not a valid URL"}}
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return gofrHttp.ErrorInvalidParam{Params: []string{"url scheme must be http or https"}}
	}

	host := parsed.Hostname()
	if host == "" {
		return gofrHttp.ErrorInvalidParam{Params: []string{"url must include a host"}}
	}

	if isDisallowedHost(host) {
		return gofrHttp.ErrorInvalidParam{Params: []string{"url must not target localhost or a private network"}}
	}

	return nil
}

func isDisallowedHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		// Not an IP literal; DNS-resolved hosts are checked at fetch time.
		return false
	}

	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
