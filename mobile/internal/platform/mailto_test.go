package platform

import "testing"

func TestParseMailto(t *testing.T) {
	to, q := parseMailto("mailto:gcationadmin27@gmail.com?subject=Payment%20for%20A%20%26%20B&body=Hi%2C%0AStudent%20ID%3A%204")
	if to != "gcationadmin27@gmail.com" || q.Get("subject") != "Payment for A & B" || q.Get("body") != "Hi,\nStudent ID: 4" {
		t.Fatalf("got %q %q %q", to, q.Get("subject"), q.Get("body"))
	}
	if got := gmailWeb("mailto:a@b.com?subject=Hi%20there"); got != "https://mail.google.com/mail/?view=cm&fs=1&to=a%40b.com&su=Hi+there&body=" {
		t.Fatalf("gmailWeb: %s", got)
	}
}
