package httpx

import (
	"net/http/cookiejar"
	"testing"
)

func newJar(t *testing.T) *cookiejar.Jar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return jar
}
