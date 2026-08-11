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
