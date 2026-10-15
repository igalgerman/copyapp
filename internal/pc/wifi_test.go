package pc

import "testing"

func TestParseSSID(t *testing.T) {
	connected := "\r\nThere is 1 interface on the system: \r\n\r\n" +
		"    Name                   : ‏‏Wi-Fi 2\r\n" +
		"    State                  : connected\r\n" +
		"    SSID                   : Home Wi-Fi\r\n" +
		"    AP BSSID               : aa:bb:cc:dd:ee:ff\r\n" +
		"    Profile                : Home Wi-Fi \r\n"
	cases := []struct{ name, in, want string }{
		{"connected", connected, "Home Wi-Fi"},
		{"bssid first", "    BSSID : aa:bb:cc:dd:ee:ff\n    SSID : Home 5G\n", "Home 5G"},
		{"colon in name", "    SSID                   : Cafe: guest\n", "Cafe: guest"},
		{"hebrew + rlm", "    SSID : ‏הבית של משה\r\n", "הבית של משה"},
		{"localized labels", "    Состояние : подключено\n    SSID : Дом\n", "Дом"},
		{"disconnected", "There is 1 interface on the system:\n    Name : Wi-Fi\n    State : disconnected\n", ""},
		{"no wlan service", "The Wireless AutoConfig Service (wlansvc) is not running.\n", ""},
		{"empty ssid", "    SSID : \n", ""},
		{"two interfaces", "    SSID : First\n\n    SSID : Second\n", "First"},
		{"control chars", "    SSID : a\x07b\n", "ab"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := ParseSSID(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if got := ParseSSID("SSID : " + "abcdefghijklmnopqrstuvwxyz0123456789"); len([]rune(got)) != 32 {
		t.Errorf("long SSID not capped: %q", got)
	}
}
