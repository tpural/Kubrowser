package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	Pod    PodConfig
	Auth   AuthConfig
	K8s    K8sConfig
	Server ServerConfig
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	GitHubClientID     string
	GitHubClientSecret string
	SessionSecret      string
	BaseURL            string
	AllowedUsers       []string
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// K8sConfig holds Kubernetes client configuration.
type K8sConfig struct {
	KubeconfigPath    string
	KubeconfigContent string
	Namespace         string
}

// PodConfig holds pod-related configuration.
type PodConfig struct {
	ResourceLimits     ResourceLimits
	Image              string
	Namespace          string
	ServiceAccount     string
	SessionTimeout     time.Duration
	MaxSessionsPerUser int
}

// ResourceLimits holds resource limit configuration.
type ResourceLimits struct {
	CPU    string
	Memory string
}

// Load loads configuration from environment variables with defaults.
func Load() *Config {
	// Attempt to load .env file from the current directory or parent directory.
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ReadTimeout:  getDurationEnv("READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getDurationEnv("WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getDurationEnv("IDLE_TIMEOUT", 60*time.Second),
		},
		K8s: K8sConfig{
			KubeconfigPath:    getKubeconfigPath(),
			KubeconfigContent: getEnv("KUBECONFIG_CONTENT", ""),
			Namespace:         getEnv("POD_NAMESPACE", "default"),
		},
		Pod: PodConfig{
			Image:              getEnv("POD_IMAGE", "bitnami/kubectl:latest"),
			Namespace:          getEnv("POD_NAMESPACE", "default"),
			ServiceAccount:     getEnv("POD_SERVICE_ACCOUNT", "kubectl-pod"),
			SessionTimeout:     getDurationEnv("SESSION_TIMEOUT", 60*time.Minute), // Default 1 hour.
			MaxSessionsPerUser: getIntEnv("MAX_SESSIONS_PER_USER", 5),
			ResourceLimits: ResourceLimits{
				CPU:    getEnv("POD_CPU_LIMIT", "500m"),
				Memory: getEnv("POD_MEMORY_LIMIT", "512Mi"),
			},
		},
		Auth: AuthConfig{
			GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
			GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
			SessionSecret:      getEnv("SESSION_SECRET", "change-me-in-production-secret-key-must-be-32-bytes"),
			AllowedUsers:       getStringSliceEnv("ALLOWED_USERS", []string{"tpural", "gregyjames"}),
			BaseURL:            getEnv("BASE_URL", "http://localhost:8080"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getKubeconfigPath() string {
	// 1. Check KUBECONFIG_PATH (backward compatibility).
	if path := os.Getenv("KUBECONFIG_PATH"); path != "" {
		return path
	}
	// 2. Check standard KUBECONFIG.
	if path := os.Getenv("KUBECONFIG"); path != "" {
		return path
	}
	// 3. Fallback to default (~/.kube/config).
	return getDefaultKubeconfigPath()
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getDefaultKubeconfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")
	// Check if file exists.
	if _, err := os.Stat(kubeconfigPath); err == nil {
		return kubeconfigPath
	}
	return ""
}

func getStringSliceEnv(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
