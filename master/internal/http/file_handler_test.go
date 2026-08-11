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
