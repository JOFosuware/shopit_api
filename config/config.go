package config

import (
	"errors"
	"log"
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
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, errors.New("config file not found")
		}
		return nil, err
	}

	return v, nil
}

// ParseConfig Parse config file
func ParseConfig(v *viper.Viper) (*Config, error) {
	var c Config

	err := v.Unmarshal(&c)
	if err != nil {
		log.Printf("unable to decode into struct, %v", err)
		return nil, err
	}

	return &c, nil
}
