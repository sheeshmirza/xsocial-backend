package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// weakSecrets is a set of known-dangerous placeholder values that must never
// be used in production. LoadConfig will fatal if any JWT secret matches.
var weakSecrets = map[string]bool{
	"your_jwt_access_secret_here_change_in_production":   true,
	"your_jwt_refresh_secret_here_change_in_production":  true,
	"secret":       true,
	"changeme":     true,
	"test":         true,
	"password":     true,
	"jwt_secret":   true,
}

type OAuthPlatformConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Version      string
	Scopes       string
}

type Config struct {
	Port                int
	ApiPrefix           string
	PublicBaseURL       string
	NodeEnv             string
	MongoURI            string
	MongoDBName         string
	RedisURL            string
	JWTAccessSecret     string
	JWTRefreshSecret    string
	JWTAccessExpiresIn  time.Duration
	JWTRefreshExpiresIn time.Duration
	BcryptSaltRounds    int
	ClientURL           string
	ClientOrigins       []string
	AIWrapperBaseURL    string
	AIWrapperTimeoutMS  time.Duration
	Instagram           OAuthPlatformConfig
	LinkedIn            OAuthPlatformConfig
	MCPOAuthIssuer         string
	MCPOAuthResource       string
	MCPOAuthSigningSecret  string
}

func cleanEnvValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		return strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	}
	return trimmed
}

func parseDuration(val string, defaultVal time.Duration) time.Duration {
	val = strings.TrimSpace(val)
	if val == "" {
		return defaultVal
	}
	if strings.HasSuffix(val, "d") {
		daysStr := strings.TrimSuffix(val, "d")
		days, err := strconv.Atoi(daysStr)
		if err == nil {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

func parseOrigins(value string) []string {
	cleaned := cleanEnvValue(value)
	if cleaned == "" {
		return []string{}
	}
	parts := strings.Split(cleaned, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimRight(strings.TrimSpace(p), "/")
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	// Validate JWT secrets before proceeding — fatal on weak/placeholder values.
	jwtAccessSecret := cleanEnvValue(os.Getenv("JWT_ACCESS_SECRET"))
	jwtRefreshSecret := cleanEnvValue(os.Getenv("JWT_REFRESH_SECRET"))
	nodeEnv := cleanEnvValue(os.Getenv("NODE_ENV"))
	if nodeEnv == "" {
		nodeEnv = "development"
	}
	if nodeEnv != "test" {
		if weakSecrets[strings.ToLower(jwtAccessSecret)] || len(jwtAccessSecret) < 32 {
			log.Fatalf("[FATAL] JWT_ACCESS_SECRET is missing, too short (<32 chars), or uses a known-weak placeholder. Set a strong random secret before starting.")
		}
		if weakSecrets[strings.ToLower(jwtRefreshSecret)] || len(jwtRefreshSecret) < 32 {
			log.Fatalf("[FATAL] JWT_REFRESH_SECRET is missing, too short (<32 chars), or uses a known-weak placeholder. Set a strong random secret before starting.")
		}
	}

	port, err := strconv.Atoi(cleanEnvValue(os.Getenv("PORT")))
	if err != nil || port <= 0 {
		port = 5000
	}

	bcryptRounds, err := strconv.Atoi(cleanEnvValue(os.Getenv("BCRYPT_SALT_ROUNDS")))
	if err != nil || bcryptRounds <= 0 {
		bcryptRounds = 12
	}

	aiTimeout, err := strconv.Atoi(cleanEnvValue(os.Getenv("AI_WRAPPER_TIMEOUT_MS")))
	if err != nil || aiTimeout <= 0 {
		aiTimeout = 30000
	}

	apiPrefix := cleanEnvValue(os.Getenv("API_PREFIX"))
	if apiPrefix == "" {
		apiPrefix = "/api/v1"
	}

	dbName := cleanEnvValue(os.Getenv("MONGODB_DB_NAME"))
	if dbName == "" {
		dbName = "xsocial"
	}

	mongoURI := cleanEnvValue(os.Getenv("MONGODB_URI"))
	if mongoURI == "" {
		mongoURI = "mongodb://127.0.0.1:27017/xsocial"
	}

	clientURL := cleanEnvValue(os.Getenv("CLIENT_URL"))
	if clientURL == "" {
		clientURL = "http://localhost:3001"
	}

	linkedInVersion := cleanEnvValue(os.Getenv("LINKEDIN_VERSION"))
	if linkedInVersion == "" {
		linkedInVersion = "202607"
	}

	mcpOAuthSigningSecret := cleanEnvValue(os.Getenv("MCP_OAUTH_SIGNING_SECRET"))
	if mcpOAuthSigningSecret == "" {
		mcpOAuthSigningSecret = cleanEnvValue(os.Getenv("JWT_ACCESS_SECRET"))
	}

	return &Config{
		Port:                port,
		ApiPrefix:           apiPrefix,
		PublicBaseURL:       cleanEnvValue(os.Getenv("PUBLIC_BASE_URL")),
		NodeEnv:             nodeEnv,
		MongoURI:            mongoURI,
		MongoDBName:         dbName,
		RedisURL:            cleanEnvValue(os.Getenv("REDIS_URL")),
		JWTAccessSecret:     jwtAccessSecret,
		JWTRefreshSecret:    jwtRefreshSecret,
		JWTAccessExpiresIn:  parseDuration(os.Getenv("JWT_ACCESS_EXPIRES_IN"), 15*time.Minute),
		JWTRefreshExpiresIn: parseDuration(os.Getenv("JWT_REFRESH_EXPIRES_IN"), 30*24*time.Hour),
		BcryptSaltRounds:    bcryptRounds,
		ClientURL:           clientURL,
		ClientOrigins:       parseOrigins(clientURL),
		AIWrapperBaseURL:    cleanEnvValue(os.Getenv("AI_WRAPPER_BASE_URL")),
		AIWrapperTimeoutMS:  time.Duration(aiTimeout) * time.Millisecond,
		Instagram: OAuthPlatformConfig{
			ClientID:     cleanEnvValue(os.Getenv("INSTAGRAM_CLIENT_ID")),
			ClientSecret: cleanEnvValue(os.Getenv("INSTAGRAM_CLIENT_SECRET")),
			RedirectURI:  cleanEnvValue(os.Getenv("INSTAGRAM_REDIRECT_URI")),
		},
		LinkedIn: OAuthPlatformConfig{
			ClientID:     cleanEnvValue(os.Getenv("LINKEDIN_CLIENT_ID")),
			ClientSecret: cleanEnvValue(os.Getenv("LINKEDIN_CLIENT_SECRET")),
			RedirectURI:  cleanEnvValue(os.Getenv("LINKEDIN_REDIRECT_URI")),
			Version:      linkedInVersion,
			Scopes:       cleanEnvValue(os.Getenv("LINKEDIN_SCOPES")),
		},
		MCPOAuthIssuer:        strings.TrimRight(cleanEnvValue(os.Getenv("MCP_OAUTH_ISSUER")), "/"),
		MCPOAuthResource:      strings.TrimRight(cleanEnvValue(os.Getenv("MCP_OAUTH_RESOURCE")), "/"),
		MCPOAuthSigningSecret: mcpOAuthSigningSecret,
	}
}
