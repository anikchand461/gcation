package ui

import "testing"

func TestFullLink(t *testing.T) {
	for in, want := range map[string]string{
		"meet.google.com/fsb-vucc-abc": "https://meet.google.com/fsb-vucc-abc",
		"https://meet.google.com/x":    "https://meet.google.com/x",
		"http://a.com/b":               "https://a.com/b",
		"HTTPS://x.com":                "https://x.com",
		"":                             "",
	} {
		if got := fullLink(in); got != want {
			t.Errorf("fullLink(%q) = %q, want %q", in, got, want)
		}
	}
}
