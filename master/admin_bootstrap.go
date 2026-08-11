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

package main

import (
	"errors"
	"log"
	"os"
	"strconv"
	"strings"

	"master/internal/db"

	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultWebUIAdminName     = "Web UI Admin"
	defaultWebUIAdminEmail    = "admin@localhost"
	defaultWebUIAdminPassword = "ChangeMeAdmin123!"
)

type webUIAdminBootstrapConfig struct {
	Name          string
	Email         string
	Password      string
	ResetPassword bool
}

func bootstrapDefaultWebUIAdmin(userDB *db.UserDB) error {
	if userDB == nil {
		return nil
	}

	cfg := loadWebUIAdminBootstrapConfig()
	if cfg.Password == "" {
		return errors.New("WEBUI_ADMIN_PASSWORD cannot be empty")
	}

	existingUser, err := userDB.GetUserByEmail(cfg.Email)
	if err == nil {
		if !cfg.ResetPassword {
			log.Printf("✓ Web UI admin user already exists for %s (password unchanged)", cfg.Email)
			return nil
		}

		hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			return hashErr
		}

		updates := bson.M{
			"password_hash": string(hashedPassword),
		}
		if strings.TrimSpace(cfg.Name) != "" && existingUser.Name != cfg.Name {
			updates["name"] = cfg.Name
		}

		if err := userDB.UpdateUser(cfg.Email, updates); err != nil {
			return err
		}

		log.Printf("✓ Reset password for existing Web UI admin user %s (WEBUI_ADMIN_RESET_PASSWORD=true)", cfg.Email)
		return nil
	}

	if !errors.Is(err, db.ErrUserNotFound) {
		return err
	}

	if err := userDB.CreateUser(cfg.Name, cfg.Email, cfg.Password); err != nil {
		if errors.Is(err, db.ErrUserAlreadyExists) {
			log.Printf("✓ Web UI admin user already exists for %s", cfg.Email)
			return nil
		}
		return err
	}

	log.Printf("✓ Bootstrapped default Web UI admin user: %s", cfg.Email)
	if cfg.Password == defaultWebUIAdminPassword {
		log.Printf("⚠️  WEBUI_ADMIN_PASSWORD not set; default admin password is in use. Change it in .env for non-local environments.")
	}

	return nil
}

func loadWebUIAdminBootstrapConfig() webUIAdminBootstrapConfig {
	name := strings.TrimSpace(os.Getenv("WEBUI_ADMIN_NAME"))
	if name == "" {
		name = defaultWebUIAdminName
	}

	email := strings.TrimSpace(os.Getenv("WEBUI_ADMIN_EMAIL"))
	if email == "" {
		email = defaultWebUIAdminEmail
	}

	password := os.Getenv("WEBUI_ADMIN_PASSWORD")
	if strings.TrimSpace(password) == "" {
		password = defaultWebUIAdminPassword
	}

	return webUIAdminBootstrapConfig{
		Name:          name,
		Email:         email,
		Password:      password,
		ResetPassword: getBoolEnvWithDefault("WEBUI_ADMIN_RESET_PASSWORD", false),
	}
}

func getBoolEnvWithDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("⚠️  Invalid bool value for %s: %s, using fallback %t", key, value, fallback)
		return fallback
	}
	return parsed
}
