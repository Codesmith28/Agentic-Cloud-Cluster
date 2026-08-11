package http

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestShouldUseSecureAuthCookieAutoMode(t *testing.T) {
	req := httptest.NewRequest("POST", "http://localhost:8080/api/auth/login", nil)
	if shouldUseSecureAuthCookie(req) {
		t.Fatalf("expected localhost HTTP request to use non-secure cookie")
	}

	req = httptest.NewRequest("POST", "https://example.com/api/auth/login", nil)
	req.TLS = &tls.ConnectionState{}
	if !shouldUseSecureAuthCookie(req) {
		t.Fatalf("expected HTTPS request to use secure cookie")
	}

	req = httptest.NewRequest("POST", "http://example.com/api/auth/login", nil)
	if !shouldUseSecureAuthCookie(req) {
		t.Fatalf("expected non-local host to default to secure cookie")
	}
}

func TestShouldUseSecureAuthCookieLoopbackDetection(t *testing.T) {
	req := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/auth/login", nil)
	if shouldUseSecureAuthCookie(req) {
		t.Fatalf("expected IPv4 loopback to use non-secure cookie")
	}

	req = httptest.NewRequest("POST", "http://[::1]:8080/api/auth/login", nil)
	if shouldUseSecureAuthCookie(req) {
		t.Fatalf("expected IPv6 loopback to use non-secure cookie")
	}
}

func TestShouldUseSecureAuthCookieForwardedProtoAndEnvOverride(t *testing.T) {
	req := httptest.NewRequest("POST", "http://example.com/api/auth/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if !shouldUseSecureAuthCookie(req) {
		t.Fatalf("expected X-Forwarded-Proto=https to use secure cookie")
	}

	t.Setenv("AUTH_COOKIE_SECURE", "false")
	if shouldUseSecureAuthCookie(req) {
		t.Fatalf("expected AUTH_COOKIE_SECURE=false override to disable secure cookie")
	}

	t.Setenv("AUTH_COOKIE_SECURE", "true")
	localReq := httptest.NewRequest("POST", "http://localhost:8080/api/auth/login", nil)
	if !shouldUseSecureAuthCookie(localReq) {
		t.Fatalf("expected AUTH_COOKIE_SECURE=true override to force secure cookie")
	}
}
