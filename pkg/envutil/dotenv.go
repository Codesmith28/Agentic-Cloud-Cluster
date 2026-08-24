package envutil

import (
	"log"

	"github.com/joho/godotenv"
)

// LoadDotEnv searches for and loads the nearest .env file.
func LoadDotEnv() {
	paths := []string{".env", "../.env", "../../.env"}
	for _, path := range paths {
		if err := godotenv.Load(path); err == nil {
			log.Printf("Loaded .env from %s", path)
			return
		}
	}
	log.Println("No .env file found, using environment variables")
}
