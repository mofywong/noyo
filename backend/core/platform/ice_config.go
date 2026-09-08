package platform

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// ValidateICEURLs rejects non-ICE URLs and embedded credentials before they can
// enter signaling or configuration responses. Credentials have separate fields.
func ValidateICEURLs(value string, turn bool) error {
	for _, raw := range strings.FieldsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == ',' }) {
		scheme, rest, ok := strings.Cut(raw, ":")
		validScheme := scheme == "stun" || scheme == "stuns"
		if turn {
			validScheme = scheme == "turn" || scheme == "turns"
		}
		if !ok || !validScheme || rest == "" || strings.ContainsAny(rest, "/@#") {
			return fmt.Errorf("invalid ICE server URL")
		}
		u, err := url.Parse("https://" + rest)
		if err != nil || u.Hostname() == "" || u.User != nil {
			return fmt.Errorf("invalid ICE server URL")
		}
		if p := u.Port(); p != "" {
			n, err := strconv.Atoi(p)
			if err != nil || n < 1 || n > 65535 {
				return fmt.Errorf("invalid ICE server port")
			}
		}
		query, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return fmt.Errorf("invalid ICE server transport")
		}
		if len(query) > 0 {
			transport := query.Get("transport")
			if !turn || len(query) != 1 || len(query["transport"]) != 1 || (transport != "udp" && transport != "tcp") {
				return fmt.Errorf("invalid ICE server transport")
			}
		}
	}
	return nil
}

// BuildICEServerConfigs uses the same URL-list rules for local and gateway media.
func BuildICEServerConfigs(stunURLs, turnURLs, username, password string) ([]ICEServerConfig, error) {
	servers := make([]ICEServerConfig, 0)
	seen := make(map[string]bool)
	for _, raw := range []string{stunURLs, turnURLs} {
		for _, url := range strings.FieldsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || r == ',' }) {
			if seen[url] {
				continue
			}
			seen[url] = true
			server := ICEServerConfig{URLs: []string{url}}
			if strings.HasPrefix(url, "turn:") || strings.HasPrefix(url, "turns:") {
				if username == "" || password == "" {
					return nil, fmt.Errorf("TURN username and password are required")
				}
				server.Username, server.Credential = username, password
			}
			servers = append(servers, server)
		}
	}
	return servers, nil
}
