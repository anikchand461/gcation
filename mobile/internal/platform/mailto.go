package platform

import (
	"net/url"
	"strings"
)

// parseMailto splits "mailto:a@b.com?subject=..&body=.." into the address and its query values.
func parseMailto(link string) (to string, q url.Values) {
	rest := strings.TrimPrefix(link, "mailto:")
	addr, query, _ := strings.Cut(rest, "?")
	to, _ = url.PathUnescape(addr)
	q, _ = url.ParseQuery(query)
	return to, q
}

// gmailWeb turns a mailto: link into Gmail's compose page, for devices without an email app.
func gmailWeb(link string) string {
	to, q := parseMailto(link)
	return "https://mail.google.com/mail/?view=cm&fs=1&to=" + url.QueryEscape(to) +
		"&su=" + url.QueryEscape(q.Get("subject")) + "&body=" + url.QueryEscape(q.Get("body"))
}
