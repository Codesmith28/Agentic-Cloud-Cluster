// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
