package httpx

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

// Field limits are enforced server-side and mirrored in the admin UI.
const (
	maxDisplayName = 60
	maxTagline     = 120
	maxBio         = 400
	maxLinkTitle   = 80
	maxLinkURL     = 2048
	maxIconRunes   = 8
	maxUsername    = 64
	maxHandle      = 32
	minHandle      = 2
	maxPassword    = 72 // bcrypt's input limit
)

// reservedHandles are paths the application itself owns; a profile handle may
// not shadow them. Keep this in sync with the frontend routes.
var reservedHandles = map[string]bool{
	"admin": true, "api": true, "uploads": true, "assets": true,
	"privacy": true, "terms": true, "notice": true, "legal": true,
	"about": true, "contact": true, "login": true, "logout": true,
	"health": true, "robots.txt": true, "favicon.ico": true, "index.html": true,
	"site.webmanifest": true, "og.png": true, "apple-touch-icon.png": true,
	"support": true, "help": true, "settings": true, "static": true,
}

// validateHandle normalises and checks a profile handle (the path segment the
// public page is served from).
func validateHandle(raw string) (string, string) {
	handle := strings.ToLower(strings.TrimSpace(raw))
	if handle == "" {
		return "", "A page handle is required."
	}
	if n := utf8.RuneCountInString(handle); n < minHandle || n > maxHandle {
		return "", "Must be between 2 and 32 characters."
	}
	for i := 0; i < len(handle); i++ {
		c := handle[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-' || c == '_':
			if i == 0 || i == len(handle)-1 {
				return "", "Cannot start or end with - or _."
			}
		default:
			return "", "Use lowercase letters, digits, - and _ only."
		}
	}
	if reservedHandles[handle] {
		return "", "That handle is reserved."
	}
	return handle, ""
}

// allowedLinkSchemes covers the destinations a bio page realistically needs.
var allowedLinkSchemes = map[string]bool{
	"http":   true,
	"https":  true,
	"mailto": true,
	"tel":    true,
}

// sanitiseText normalises user text: trims, collapses CR/LF and strips control
// characters that would otherwise end up verbatim in the DOM.
func sanitiseText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// checkLen records a field error when a string exceeds its rune budget.
func checkLen(fields map[string]string, name, value string, max int) {
	if utf8.RuneCountInString(value) > max {
		fields[name] = "Must be at most " + itoa(max) + " characters."
	}
}

// validateAccountName checks a login name. Account names are not part of any
// URL, so the rules are only about being typeable and unambiguous.
func validateAccountName(raw string) (string, string) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", "A username is required."
	}
	if n := utf8.RuneCountInString(name); n < 2 || n > maxUsername {
		return "", "Must be between 2 and 64 characters."
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return "", "Use letters, digits, and . _ - only."
		}
	}
	return name, ""
}

// validateLinkURL normalises and validates a destination URL.
func validateLinkURL(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "A URL is required."
	}
	if utf8.RuneCountInString(raw) > maxLinkURL {
		return "", "URL is too long."
	}

	// Bare domains typed into the admin form default to https.
	if !strings.Contains(raw, ":") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", "That is not a valid URL."
	}
	scheme := strings.ToLower(u.Scheme)
	if !allowedLinkSchemes[scheme] {
		return "", "Only http, https, mailto and tel links are allowed."
	}
	if (scheme == "http" || scheme == "https") && u.Host == "" {
		return "", "That URL is missing a host name."
	}
	if (scheme == "mailto" || scheme == "tel") && u.Opaque == "" {
		return "", "That address is incomplete."
	}
	u.Scheme = scheme
	return u.String(), ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
