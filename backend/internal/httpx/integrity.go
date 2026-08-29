package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// Analytics integrity: the counters are only worth reading if they cannot be
// inflated by whoever they flatter. Three cheap checks, applied in order:
//
//  1. the request has to look like it came from this site's own page,
//  2. obvious bots and command-line clients are ignored,
//  3. the same visitor is counted at most once per window (store.ClaimEvent).
//
// None of them are individually unbeatable — farming still needs a rotating
// address pool and a browser-shaped client — but together they remove every
// trivial way of running the number up.

// botAgents are substrings that mark a non-human client. Matched lowercase.
var botAgents = []string{
	"bot", "crawl", "spider", "slurp", "archiver", "headless", "phantom",
	"curl/", "wget", "python-requests", "python-urllib", "go-http-client",
	"okhttp", "libwww", "httpclient", "postman", "insomnia", "lighthouse",
	"pingdom", "uptime", "monitor", "preview", "scanner", "facebookexternalhit",
	"embedly", "quora link preview", "whatsapp", "telegrambot", "discordbot",
}

// looksAutomated reports whether the user agent should be excluded from counts.
// A missing user agent counts as automated: browsers always send one.
func looksAutomated(userAgent string) bool {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return true
	}
	for _, needle := range botAgents {
		if strings.Contains(ua, needle) {
			return true
		}
	}
	return false
}

// fromOwnPage reports whether a beacon plausibly came from this site's page.
//
// Browsers send Sec-Fetch-Site on every fetch; where it is missing (older
// browsers) the Origin or Referer header is used instead. A bare POST with no
// context at all — the shape of a shell loop — does not qualify.
func (s *Server) fromOwnPage(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin" || site == "none"
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		if ref := r.Header.Get("Referer"); ref != "" {
			if idx := strings.Index(ref, "://"); idx > 0 {
				rest := ref[idx+3:]
				if slash := strings.IndexByte(rest, '/'); slash >= 0 {
					rest = rest[:slash]
				}
				origin = ref[:idx+3] + rest
			}
		}
	}
	return origin != "" && s.originAllowed(r, origin)
}

// fingerprint derives the per-visitor key used to de-duplicate counting.
//
// It is an HMAC over the client address and user agent under a server-side
// secret that never leaves the database, truncated to 128 bits. It cannot be
// reversed into an address, it is scoped so a page view and a click on the same
// link do not collide, and the row it keys expires within hours.
func (s *Server) fingerprint(r *http.Request, scope string) string {
	mac := hmac.New(sha256.New, s.eventSecret)
	mac.Write([]byte(scope))
	mac.Write([]byte{0})
	mac.Write([]byte(s.clientIP(r)))
	mac.Write([]byte{0})
	mac.Write([]byte(r.UserAgent()))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}
