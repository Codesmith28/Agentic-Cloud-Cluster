package app

import (
	"errors"
	"log"
	"strings"

	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/envutil"
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

		log.Printf("✓ Web UI admin user password reset for %s", cfg.Email)
		return nil
	}

	if !errors.Is(err, db.ErrUserNotFound) {
		return err
	}

	if err := userDB.CreateUser(cfg.Name, cfg.Email, cfg.Password); err != nil {
		return err
	}

	log.Printf("✓ Default Web UI admin user bootstrapped for %s", cfg.Email)
	return nil
}

func loadWebUIAdminBootstrapConfig() webUIAdminBootstrapConfig {
	name := strings.TrimSpace(envutil.GetEnv("WEBUI_ADMIN_NAME", defaultWebUIAdminName))
	email := strings.TrimSpace(envutil.GetEnv("WEBUI_ADMIN_EMAIL", defaultWebUIAdminEmail))
	password := strings.TrimSpace(envutil.GetEnv("WEBUI_ADMIN_PASSWORD", defaultWebUIAdminPassword))
	resetPassword := envutil.GetEnvBool("WEBUI_ADMIN_RESET_PASSWORD", false)

	return webUIAdminBootstrapConfig{
		Name:          name,
		Email:         email,
		Password:      password,
		ResetPassword: resetPassword,
	}
}
