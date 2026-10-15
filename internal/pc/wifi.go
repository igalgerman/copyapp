package pc

import (
	"strings"
	"unicode"
)

// ParseSSID returns the SSID of the first connected interface in the output
// of `netsh wlan show interfaces`, or "" (no Wi-Fi, disconnected, wired only).
// Only the line whose label is exactly "SSID" counts ("AP BSSID" / "BSSID"
// do not). The labels other than SSID are localized by Windows, so the parser
// relies on nothing else.
func ParseSSID(out string) string {
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || cleanSSID(k) != "SSID" {
			continue
		}
		if s := cleanSSID(v); s != "" {
			return s
		}
	}
	return ""
}

// cleanSSID drops control and invisible format characters (netsh adds RLM
// marks on RTL systems) and caps the length at 32 bytes' worth of runes.
func cleanSSID(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 32 {
		s = string(r[:32])
	}
	return s
}
