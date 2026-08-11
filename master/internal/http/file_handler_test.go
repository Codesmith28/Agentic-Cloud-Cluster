package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListFilesRejectsLegacyIdentityQuery(t *testing.T) {
	handler := &FileAPIHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/files?user_id=alice&requesting_user=bob", nil)
	req = req.WithContext(withAuthPrincipal(req.Context(), &AuthPrincipal{
		Email: "alice@example.com",
		Role:  "user",
	}))

	rr := httptest.NewRecorder()
	handler.HandleListFiles(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
