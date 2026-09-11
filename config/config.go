package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all environment-driven configuration for the RideX Angola platform.
type Config struct {
	// Environment controls safety checks for secrets and other defaults.
	Environment string

	// Server
	ServerPort            string
	ServerReadTimeout     time.Duration
	ServerWriteTimeout    time.Duration
	ServerShutdownTimeout time.Duration
	CORSAllowedOrigins    []string
	CORSAllowCredentials  bool
	MaxRequestBodyBytes   int64

	// Database
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	// Valkey
	ValkeyHost     string
	ValkeyPort     string
	ValkeyPassword string
	ValkeyDB       int
	ValkeyPoolSize int

	// JWT
	JWTAccessSecret     string
	JWTRefreshSecret    string
	JWTAccessExpirySec  time.Duration
	JWTRefreshExpirySec time.Duration

	// SMS / OTP
	SMSProvider      string
	SMSOTP_TEMPLATE  string
	OTPCodeExpirySec time.Duration
	OTPCodeLength    int

	// Africa's Talking (SMS provider option)
	ATSciptKey string
	ATUsername string

	// Termii (SMS provider option)
	TermiiAPIKey   string
	TermiiSenderID string

	// Twilio (SMS provider option)
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFrom       string

	// Push notifications
	PushProvider   string
	FCMProjectID   string
	FCMCredentials string // path to service-account JSON or inline JSON
	APNSTeamID     string
	APNSKeyID      string
	APNSBundleID   string
	APNSPrivateKey string // path to .p8 key or inline PEM
	APNSProduction bool

	// Google Maps
	GoogleMapsAPIKey   string
	GoogleMapsFallback string

	// Payment
	PaymentProvider      string
	PaymentWebhookSecret string

	// Payout execution. "manual" uses the in-process executor (submissions are
	// recorded; no real money moves). Empty/"none" disables the worker jobs.
	PayoutExecutor string

	// AppyPay
	AppyPayClientID     string
	AppyPayClientSecret string
	AppyPayBaseURL      string
	AppyPayGPOEnabled   bool

	// VPOS
	VPOSDeveloperID   string
	VPOSAPIKey        string
	VPOSBaseURL       string
	VPOSWebhookSecret string

	// ProxyPay
	ProxyPayBaseURL string
	ProxyPayAPIKey  string

	// Asynq
	AsynqConsistencyDir    string
	AsynqLogFormat         string
	AsynqMaxProcessingTime time.Duration

	// gRPC
	GRPCPort              string
	GoogleMapsServiceAddr string
	GoogleMapsServicePort string

	// Logging
	LogLevel  string
	LogFormat string
}

// Load reads configuration from environment variables and .env files.
func Load() (*Config, error) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/app")

	setDefaults()
	_ = viper.ReadInConfig()

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	c := &Config{}

	c.Environment = viper.GetString("APP_ENV")
	c.ServerPort = viper.GetString("SERVER_PORT")
	c.ServerReadTimeout = viper.GetDuration("SERVER_READ_TIMEOUT_SEC") * time.Second
	c.ServerWriteTimeout = viper.GetDuration("SERVER_WRITE_TIMEOUT_SEC") * time.Second
	c.ServerShutdownTimeout = viper.GetDuration("SERVER_SHUTDOWN_TIMEOUT_SEC") * time.Second
	c.CORSAllowedOrigins = splitCSV(viper.GetString("CORS_ALLOWED_ORIGINS"))
	c.CORSAllowCredentials = viper.GetBool("CORS_ALLOW_CREDENTIALS")
	c.MaxRequestBodyBytes = viper.GetInt64("MAX_REQUEST_BODY_BYTES")

	c.DBHost = viper.GetString("DB_HOST")
	c.DBPort = viper.GetString("DB_PORT")
	c.DBUser = viper.GetString("DB_USER")
	c.DBPassword = viper.GetString("DB_PASSWORD")
	c.DBName = viper.GetString("DB_NAME")
	c.DBSSLMode = viper.GetString("DB_SSL_MODE")
	c.DBMaxOpenConns = viper.GetInt("DB_MAX_OPEN_CONNS")
	c.DBMaxIdleConns = viper.GetInt("DB_MAX_IDLE_CONNS")
	c.DBConnMaxLifetime = viper.GetDuration("DB_CONN_MAX_LIFETIME_SEC") * time.Second

	c.ValkeyHost = viper.GetString("VALKEY_HOST")
	c.ValkeyPort = viper.GetString("VALKEY_PORT")
	c.ValkeyPassword = viper.GetString("VALKEY_PASSWORD")
	c.ValkeyDB = viper.GetInt("VALKEY_DB")
	c.ValkeyPoolSize = viper.GetInt("VALKEY_POOL_SIZE")

	c.JWTAccessSecret = viper.GetString("JWT_ACCESS_SECRET")
	c.JWTRefreshSecret = viper.GetString("JWT_REFRESH_SECRET")
	c.JWTAccessExpirySec = viper.GetDuration("JWT_ACCESS_EXPIRY_SEC") * time.Second
	c.JWTRefreshExpirySec = viper.GetDuration("JWT_REFRESH_EXPIRY_SEC") * time.Second

	c.SMSProvider = viper.GetString("SMS_PROVIDER")
	c.SMSOTP_TEMPLATE = viper.GetString("SMS_TEMPLATE")
	c.OTPCodeExpirySec = viper.GetDuration("OTP_EXPIRY_SEC") * time.Second
	c.OTPCodeLength = viper.GetInt("OTP_CODE_LENGTH")

	c.ATSciptKey = viper.GetString("AT_SCRIPT_KEY")
	c.ATUsername = viper.GetString("AT_USERNAME")

	c.TermiiAPIKey = viper.GetString("TERMII_API_KEY")
	c.TermiiSenderID = viper.GetString("TERMII_SENDER_ID")

	c.TwilioAccountSID = viper.GetString("TWILIO_ACCOUNT_SID")
	c.TwilioAuthToken = viper.GetString("TWILIO_AUTH_TOKEN")
	c.TwilioFrom = viper.GetString("TWILIO_FROM")

	c.PushProvider = viper.GetString("PUSH_PROVIDER")
	c.FCMProjectID = viper.GetString("FCM_PROJECT_ID")
	c.FCMCredentials = viper.GetString("FCM_CREDENTIALS")
	c.APNSTeamID = viper.GetString("APNS_TEAM_ID")
	c.APNSKeyID = viper.GetString("APNS_KEY_ID")
	c.APNSBundleID = viper.GetString("APNS_BUNDLE_ID")
	c.APNSPrivateKey = viper.GetString("APNS_PRIVATE_KEY")
	c.APNSProduction = viper.GetBool("APNS_PRODUCTION")

	c.GoogleMapsAPIKey = viper.GetString("GOOGLE_MAPS_API_KEY")
	c.GoogleMapsFallback = viper.GetString("GOOGLE_MAPS_FALLBACK")

	c.PaymentProvider = viper.GetString("PAYMENT_PROVIDER")
	c.PaymentWebhookSecret = viper.GetString("PAYMENT_WEBHOOK_SECRET")
	c.PayoutExecutor = viper.GetString("PAYOUT_EXECUTOR")

	c.AppyPayClientID = viper.GetString("APPYPAY_CLIENT_ID")
	c.AppyPayClientSecret = viper.GetString("APPYPAY_CLIENT_SECRET")
	c.AppyPayBaseURL = viper.GetString("APPYPAY_BASE_URL")
	c.AppyPayGPOEnabled = viper.GetBool("APPYPAY_GPO_ENABLED")

	c.VPOSDeveloperID = viper.GetString("VPOS_DEVELOPER_ID")
	c.VPOSAPIKey = viper.GetString("VPOS_API_KEY")
	c.VPOSBaseURL = viper.GetString("VPOS_BASE_URL")
	c.VPOSWebhookSecret = viper.GetString("VPOS_WEBHOOK_SECRET")

	c.ProxyPayBaseURL = viper.GetString("PROXYPAY_BASE_URL")
	c.ProxyPayAPIKey = viper.GetString("PROXYPAY_API_KEY")

	c.AsynqConsistencyDir = viper.GetString("ASYNQ_CONSISTENCY_DIR")
	c.AsynqLogFormat = viper.GetString("ASYNQ_LOG_FORMAT")
	c.AsynqMaxProcessingTime = viper.GetDuration("ASYNQ_MAX_PROCESSING_TIME_SEC") * time.Second

	c.GRPCPort = viper.GetString("GRPC_PORT")
	c.GoogleMapsServiceAddr = viper.GetString("GRPC_GOOGLE_MAPS_SERVICE_ADDR")
	c.GoogleMapsServicePort = viper.GetString("GRPC_GOOGLE_MAPS_SERVICE_PORT")

	c.LogLevel = viper.GetString("LOG_LEVEL")
	c.LogFormat = viper.GetString("LOG_FORMAT")

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func setDefaults() {
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("SERVER_READ_TIMEOUT_SEC", 15)
	viper.SetDefault("SERVER_WRITE_TIMEOUT_SEC", 15)
	viper.SetDefault("SERVER_SHUTDOWN_TIMEOUT_SEC", 30)
	viper.SetDefault("CORS_ALLOWED_ORIGINS", "")
	viper.SetDefault("CORS_ALLOW_CREDENTIALS", false)
	viper.SetDefault("MAX_REQUEST_BODY_BYTES", 1048576)
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_USER", "ridex")
	viper.SetDefault("DB_PASSWORD", "ridex_secret")
	viper.SetDefault("DB_NAME", "ridex")
	viper.SetDefault("DB_SSL_MODE", "disable")
	viper.SetDefault("DB_MAX_OPEN_CONNS", 25)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DB_CONN_MAX_LIFETIME_SEC", 300)
	viper.SetDefault("VALKEY_HOST", "localhost")
	viper.SetDefault("VALKEY_PORT", "6379")
	viper.SetDefault("VALKEY_PASSWORD", "")
	viper.SetDefault("VALKEY_DB", 0)
	viper.SetDefault("VALKEY_POOL_SIZE", 20)
	viper.SetDefault("JWT_ACCESS_SECRET", "dev-access-change-me")
	viper.SetDefault("JWT_REFRESH_SECRET", "dev-refresh-change-me")
	viper.SetDefault("JWT_ACCESS_EXPIRY_SEC", 900)
	viper.SetDefault("JWT_REFRESH_EXPIRY_SEC", 604800)
	// OTP delivery is intentionally disabled until a documented provider is
	// selected and implemented behind auth.OTPDelivery.
	viper.SetDefault("SMS_PROVIDER", "none")
	viper.SetDefault("SMS_TEMPLATE", "")
	viper.SetDefault("OTP_EXPIRY_SEC", 300)
	viper.SetDefault("OTP_CODE_LENGTH", 6)
	viper.SetDefault("TERMII_SENDER_ID", "RideX")
	viper.SetDefault("PAYMENT_PROVIDER", "appypay")
	viper.SetDefault("PAYMENT_WEBHOOK_SECRET", "")
	viper.SetDefault("APPYPAY_BASE_URL", "https://sandbox.appypay.co")
	viper.SetDefault("APPYPAY_GPO_ENABLED", true)
	viper.SetDefault("VPOS_BASE_URL", "https://api.vpos.ao")
	viper.SetDefault("PROXYPAY_BASE_URL", "https://api.proxypay.co")
	viper.SetDefault("ASYNQ_CONSISTENCY_DIR", "/tmp/asynq")
	viper.SetDefault("ASYNQ_LOG_FORMAT", "text")
	viper.SetDefault("ASYNQ_MAX_PROCESSING_TIME_SEC", 3600)
	viper.SetDefault("GRPC_PORT", "9090")
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("LOG_FORMAT", "console")
}

// Validate rejects unsafe development defaults when running outside a local
// development environment. This is intentionally performed during startup so
// a misconfigured deployment fails closed instead of accepting real traffic.
func (c *Config) Validate() error {
	env := strings.ToLower(strings.TrimSpace(c.Environment))
	if env == "" {
		env = "development"
		c.Environment = env
	}
	if c.MaxRequestBodyBytes <= 0 {
		return fmt.Errorf("MAX_REQUEST_BODY_BYTES must be greater than zero")
	}
	if env == "development" || env == "dev" || env == "test" {
		return nil
	}

	for name, value := range map[string]string{
		"JWT_ACCESS_SECRET":  c.JWTAccessSecret,
		"JWT_REFRESH_SECRET": c.JWTRefreshSecret,
	} {
		if isUnsafeSecret(value) || len([]byte(value)) < 32 {
			return fmt.Errorf("%s must be a random secret of at least 32 bytes outside development", name)
		}
	}
	if c.JWTAccessSecret == c.JWTRefreshSecret {
		return fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must be different")
	}
	if c.DBPassword == "ridex_secret" || isUnsafeSecret(c.DBPassword) {
		return fmt.Errorf("DB_PASSWORD must be changed outside development")
	}
	if strings.TrimSpace(c.PaymentProvider) != "" && strings.ToLower(c.PaymentProvider) != "none" &&
		(isUnsafeSecret(c.PaymentWebhookSecret) || len([]byte(c.PaymentWebhookSecret)) < 32) {
		return fmt.Errorf("PAYMENT_WEBHOOK_SECRET must be configured with at least 32 bytes when payments are enabled")
	}
	if payoutExecutor := strings.ToLower(strings.TrimSpace(c.PayoutExecutor)); payoutExecutor != "" && payoutExecutor != "none" && payoutExecutor != "manual" {
		return fmt.Errorf("PAYOUT_EXECUTOR must be 'manual' when set (got %q)", payoutExecutor)
	}
	if c.CORSAllowCredentials {
		for _, origin := range c.CORSAllowedOrigins {
			if origin == "*" {
				return fmt.Errorf("CORS_ALLOWED_ORIGINS cannot contain '*' when credentials are enabled")
			}
		}
	}
	return nil
}

func isUnsafeSecret(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "change-me", "change-this-to-a-random-256-bit-secret",
		"change-this-to-a-different-random-256-bit-secret",
		"dev-access-change-me", "dev-refresh-change-me",
		"dev-access-secret-please-change", "dev-refresh-secret-please-change":
		return true
	default:
		return false
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// DSN returns the PostgreSQL connection string for pgx.
func (c *Config) DSN() string {
	sslMode := c.DBSSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	dsn := postgresDSN(c, sslMode)
	if dsn != "" {
		return dsn
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, sslMode,
	)
}

func postgresDSN(c *Config, sslMode string) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   net.JoinHostPort(c.DBHost, c.DBPort),
		Path:   "/" + c.DBName,
	}
	query := u.Query()
	query.Set("sslmode", sslMode)
	u.RawQuery = query.Encode()
	return u.String()
}

// ValkeyAddr returns the Valkey/Redis address string.
func (c *Config) ValkeyAddr() string {
	return fmt.Sprintf("%s:%s", c.ValkeyHost, c.ValkeyPort)
}
