package validator

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/monishpn/page_pulse/models"
)

const maxURLLength = 2048

// ValidateURL enforces FR-2: the url must be non-empty, at most 2048
// characters, http/https only, and must not target localhost or a
// private network.
func ValidateURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return &models.CustomError{Message: "url is required", Code: http.StatusBadRequest}
	}

	if len(raw) > maxURLLength {
		return &models.CustomError{Message: "url must not exceed 2048 characters", Code: http.StatusBadRequest}
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return &models.CustomError{Message: "url is not a valid URL", Code: http.StatusBadRequest}
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return &models.CustomError{Message: "url scheme must be http or https", Code: http.StatusBadRequest}
	}

	host := parsed.Hostname()
	if host == "" {
		return &models.CustomError{Message: "url must include a host", Code: http.StatusBadRequest}
	}

	if isDisallowedHost(host) {
		return &models.CustomError{Message: "url must not target localhost or a private network", Code: http.StatusBadRequest}
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
