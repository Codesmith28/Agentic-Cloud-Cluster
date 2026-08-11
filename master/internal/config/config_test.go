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

package config

import "testing"

func TestNormalizePPOMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "active", in: "active", want: "active"},
		{name: "shadow uppercase", in: "SHADOW", want: "shadow"},
		{name: "fallback spaced", in: " fallback ", want: "fallback"},
		{name: "empty defaults active", in: "", want: "active"},
		{name: "invalid defaults active", in: "unknown", want: "active"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePPOMode(tt.in); got != tt.want {
				t.Fatalf("normalizePPOMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
