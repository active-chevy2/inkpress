package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DBHost     string
	DBPort     int
	DBUser     string
	DBPass     string
	DBName     string
	SecretKey  string
	Port       string
	UploadsDir string
	BaseURL    string
}

func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName)
}

func Load() *Config {
	port, _ := strconv.Atoi(getEnv("DB_PORT", "3306"))
	return &Config{
		DBHost:     getEnv("DB_HOST", "127.0.0.1"),
		DBPort:     port,
		DBUser:     getEnv("DB_USER", "inkpress"),
		DBPass:     getEnv("DB_PASS", "inkpress"),
		DBName:     getEnv("DB_NAME", "inkpress"),
		SecretKey:  getEnv("SECRET_KEY", "change-me-in-production-please-use-a-long-random-string"),
		Port:       getEnv("PORT", "8080"),
		UploadsDir: getEnv("UPLOADS_DIR", "web/static/uploads"),
		BaseURL:    strings.TrimRight(getEnv("BASE_URL", "http://localhost:8080"), "/"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
