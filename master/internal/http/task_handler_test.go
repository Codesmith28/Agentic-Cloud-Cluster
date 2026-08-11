package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleCreateTaskRejectsLegacyUserIDField(t *testing.T) {
	handler := &TaskAPIHandler{}

	body := `{
		"docker_image": "python:3.11",
		"cpu_required": 1,
		"memory_required": 1,
		"user_id": "spoofed@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
	req = req.WithContext(withAuthPrincipal(req.Context(), &AuthPrincipal{
		Email: "owner@example.com",
		Role:  "user",
	}))

	rr := httptest.NewRecorder()
	handler.HandleCreateTask(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
