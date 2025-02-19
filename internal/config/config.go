package config

import (
    "os"
    "github.com/joho/godotenv"
)

type Config struct {
    DatabaseURL    string
    ServerAddr     string
    FacebookEnabled bool
    FacebookToken  string
    FacebookPixelID string
    CloudflareToken string
    ServerIP        string
}

func Load() (*Config, error) {
    // Load .env file if it exists
    godotenv.Load()

    return &Config{
        DatabaseURL:     getEnv("DATABASE_URL", "unchained_user:dev_password@tcp(localhost:3306)/unchained_tracker"),
        ServerAddr:      getEnv("SERVER_ADDR", ":8080"),
        FacebookEnabled: getEnv("FB_ENABLED", "false") == "true",
        FacebookToken:   getEnv("FB_ACCESS_TOKEN", ""),
        FacebookPixelID: getEnv("FB_PIXEL_ID", ""),
        CloudflareToken: os.Getenv("CLOUDFLARE_TOKEN"),
        ServerIP:        os.Getenv("SERVER_IP"),
    }, nil
}

func getEnv(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return defaultValue
} 