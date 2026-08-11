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

func TestLoadConfig_UsesLegacyMongoEnvFallbacks(t *testing.T) {
	t.Setenv("MONGODB_USERNAME", "")
	t.Setenv("MONGODB_PASSWORD", "")
	t.Setenv("MONGODB_HOST", "")
	t.Setenv("MONGODB_DATABASE", "")

	t.Setenv("MONGO_USER", "legacy-user")
	t.Setenv("MONGO_PASSWORD", "legacy-pass")
	t.Setenv("MONGO_HOST", "legacy-mongo:27017")
	t.Setenv("MONGO_DATABASE", "legacy-db")

	cfg := LoadConfig()

	if cfg.MongoDBUsername != "legacy-user" {
		t.Fatalf("expected MongoDB username from MONGO_USER, got %q", cfg.MongoDBUsername)
	}
	if cfg.MongoDBPassword != "legacy-pass" {
		t.Fatalf("expected MongoDB password from MONGO_PASSWORD, got %q", cfg.MongoDBPassword)
	}
	if cfg.MongoDBDatabase != "legacy-db" {
		t.Fatalf("expected MongoDB database from MONGO_DATABASE, got %q", cfg.MongoDBDatabase)
	}
	if cfg.MongoDBURI != "mongodb://legacy-user:legacy-pass@legacy-mongo:27017" {
		t.Fatalf("unexpected MongoDB URI: %q", cfg.MongoDBURI)
	}
}

func TestLoadConfig_PrefersPrimaryMongoEnvNames(t *testing.T) {
	t.Setenv("MONGODB_USERNAME", "primary-user")
	t.Setenv("MONGODB_PASSWORD", "primary-pass")
	t.Setenv("MONGODB_HOST", "primary-mongo:27017")
	t.Setenv("MONGODB_DATABASE", "primary-db")

	t.Setenv("MONGO_USER", "legacy-user")
	t.Setenv("MONGO_PASSWORD", "legacy-pass")
	t.Setenv("MONGO_HOST", "legacy-mongo:27017")
	t.Setenv("MONGO_DATABASE", "legacy-db")

	cfg := LoadConfig()

	if cfg.MongoDBUsername != "primary-user" {
		t.Fatalf("expected MongoDB username from MONGODB_USERNAME, got %q", cfg.MongoDBUsername)
	}
	if cfg.MongoDBPassword != "primary-pass" {
		t.Fatalf("expected MongoDB password from MONGODB_PASSWORD, got %q", cfg.MongoDBPassword)
	}
	if cfg.MongoDBDatabase != "primary-db" {
		t.Fatalf("expected MongoDB database from MONGODB_DATABASE, got %q", cfg.MongoDBDatabase)
	}
	if cfg.MongoDBURI != "mongodb://primary-user:primary-pass@primary-mongo:27017" {
		t.Fatalf("unexpected MongoDB URI: %q", cfg.MongoDBURI)
	}
}
