package config

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is App config struct
type Config struct {
	Server     ServerConfig
	Postgres   PostgresConfig
	Cookie     Cookie
	Logger     Logger
	Stripe     Stripe
	SMTP       SMTP
	Cloudinary Cloudinary
	SecretKey  string
	Frontend   string
}

// ServerConfig Server config struct
type ServerConfig struct {
	AppVersion        string
	Port              string
	Mode              string
	JwtSecretKey      string
	CookieName        string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	SSL               bool
	CtxDefaultTimeout time.Duration
	CSRF              bool
	Debug             bool
}

// Logger config
type Logger struct {
	Development       bool
	DisableCaller     bool
	DisableStacktrace bool
	Encoding          string
	Level             string
}

// PostgresConfig Postgresql config
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Dbname   string
	SSLMode  string
	PgDriver string
	Url      string
}

// Cookie config
type Cookie struct {
	Name     string
	MaxAge   int
	Secure   bool
	HTTPOnly bool
}

// Stripe config
type Stripe struct {
	Secret string
	Key    string
}

// SMTP config
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
}

// Cloudinary config
type Cloudinary struct {
	Name   string
	Key    string
	Secret string
}

// LoadConfig Load config file from given path
func LoadConfig(filename string) (*viper.Viper, error) {
	v := viper.New()

	v.SetConfigName(filename)
	v.AddConfigPath(".")
	// allow env vars to override config using _ for nested keys
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// explicit bindings for commonly overridden keys (single list, no duplicates)
	v.BindEnv("server.port", "PORT")
	v.BindEnv("server.appversion", "SERVER_APPVERSION")
	v.BindEnv("server.mode", "SERVER_MODE")
	v.BindEnv("server.jwtsecretkey", "JWT_SECRET_KEY")
	v.BindEnv("server.cookiename", "COOKIE_NAME")

	v.BindEnv("postgres.url", "DATABASE_URL")

	v.BindEnv("stripe.secret", "STRIPE_SECRET")
	v.BindEnv("stripe.key", "STRIPE_KEY")

	v.BindEnv("smtp.host", "SMTP_HOST")
	v.BindEnv("smtp.port", "SMTP_PORT")
	v.BindEnv("smtp.username", "SMTP_USERNAME")
	v.BindEnv("smtp.password", "SMTP_PASSWORD")

	v.BindEnv("cloudinary.name", "CLOUDINARY_NAME")
	v.BindEnv("cloudinary.key", "CLOUDINARY_KEY")
	v.BindEnv("cloudinary.secret", "CLOUDINARY_SECRET")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// file not present — continue, we'll rely on env vars
			log.Println("config file not found, continuing with environment variables")
		} else {
			return nil, err // syntax/parse error -> fail
		}
	}

	return v, nil
}

// ParseConfig Parse config file
func ParseConfig(v *viper.Viper) (*Config, error) {
	var c Config

	// Normalize numeric timeout values (seconds) into duration strings so
	// they unmarshal properly into time.Duration fields. Accept either
	// integer seconds or duration strings like "5s" in config.
	durationKeys := []string{"server.readtimeout", "server.writetimeout", "server.ctxdefaulttimeout"}
	for _, k := range durationKeys {
		if v.IsSet(k) {
			val := v.Get(k)
			switch val.(type) {
			case int, int32, int64:
				v.Set(k, fmt.Sprintf("%ds", v.GetInt(k)))
			case float32, float64:
				// if someone used a float, treat as seconds
				v.Set(k, fmt.Sprintf("%ds", int(v.GetFloat64(k))))
			case string:
				// assume it's already a proper duration string like "5s"
			}
		}
	}

	if err := v.Unmarshal(&c); err != nil {
		log.Printf("unable to decode into struct, %v", err)
		return nil, err
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	return &c, nil
}

// Validate performs basic sanity checks on required configuration values.
func (c *Config) Validate() error {
	// JWT and DB (existing)
	if c.Server.JwtSecretKey == "" {
		return errors.New("missing required secret: set JWT_SECRET_KEY (server.jwtSecretKey)")
	}
	if c.Postgres.Url == "" {
		if c.Postgres.Host == "" || c.Postgres.User == "" || c.Postgres.Dbname == "" {
			return errors.New("missing postgres configuration: set DATABASE_URL or POSTGRES_HOST/POSTGRES_USER/POSTGRES_DB")
		}
	}

	// Payment (required in prod or when enabled)
	if c.Server.Mode != "Development" {
		if c.Stripe.Secret == "" {
			return errors.New("missing STRIPE_SECRET (stripe.secret)")
		}
		if c.Stripe.Key == "" {
			return errors.New("missing STRIPE_KEY (stripe.key)")
		}
	}

	// Cloudinary
	if c.Cloudinary.Name == "" || c.Cloudinary.Key == "" || c.Cloudinary.Secret == "" {
		return errors.New("missing cloudinary credentials: set CLOUDINARY_NAME/CLOUDINARY_KEY/CLOUDINARY_SECRET")
	}

	// SMTP
	if c.SMTP.Host == "" || c.SMTP.Port == 0 || c.SMTP.Username == "" || c.SMTP.Password == "" {
		return errors.New("incomplete SMTP configuration: set SMTP_HOST/SMTP_PORT/SMTP_USERNAME/SMTP_PASSWORD")
	}

	return nil
}
