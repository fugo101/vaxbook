// Package config reads the process configuration from environment variables.
// Real values live only in the deployment .env, never in the repo (CLAUDE.md).
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Env is the deployment environment.
type Env string

const (
	EnvDev  Env = "dev"
	EnvProd Env = "prod"
)

// MinPepperLen is the minimum length of VAXBOOK_OTP_PEPPER, in bytes.
const MinPepperLen = 32

// Config is the process configuration.
type Config struct {
	Env         Env        // VAXBOOK_ENV: dev | prod (default prod)
	HTTPAddr    string     // VAXBOOK_HTTP_ADDR (default :8080)
	DatabaseURL string     // VAXBOOK_DATABASE_URL (required)
	AppHost     string     // VAXBOOK_APP_HOST: host of the user app, e.g. localhost (required)
	OTPPepper   []byte     // VAXBOOK_OTP_PEPPER: HMAC key for OTP hashes, ≥ 32 bytes (required)
	Mailer      string     // VAXBOOK_MAILER: console (dev only); real mailers come later
	LogLevel    slog.Level // VAXBOOK_LOG_LEVEL: debug | info | warn | error (default info)
}

// FromEnv reads the configuration through getenv (os.Getenv in production).
func FromEnv(getenv func(string) string) (Config, error) {
	c := Config{
		Env:         Env(getenv("VAXBOOK_ENV")),
		HTTPAddr:    getenv("VAXBOOK_HTTP_ADDR"),
		DatabaseURL: getenv("VAXBOOK_DATABASE_URL"),
		AppHost:     strings.ToLower(strings.TrimSpace(getenv("VAXBOOK_APP_HOST"))),
		OTPPepper:   []byte(getenv("VAXBOOK_OTP_PEPPER")),
		Mailer:      getenv("VAXBOOK_MAILER"),
	}
	if c.Env == "" {
		c.Env = EnvProd
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":8080"
	}

	var errs []error
	if c.Env != EnvDev && c.Env != EnvProd {
		errs = append(errs, fmt.Errorf("VAXBOOK_ENV must be %q or %q", EnvDev, EnvProd))
	}
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("VAXBOOK_DATABASE_URL is required"))
	}
	if c.AppHost == "" {
		errs = append(errs, errors.New("VAXBOOK_APP_HOST is required"))
	} else if strings.ContainsAny(c.AppHost, ":/") {
		errs = append(errs, errors.New("VAXBOOK_APP_HOST must be a bare host name, without scheme or port"))
	}
	if len(c.OTPPepper) < MinPepperLen {
		errs = append(errs, fmt.Errorf("VAXBOOK_OTP_PEPPER must be at least %d bytes", MinPepperLen))
	}
	switch c.Mailer {
	case "console":
		if c.Env != EnvDev {
			errs = append(errs, errors.New("VAXBOOK_MAILER=console prints sign-in codes to the log and is allowed only with VAXBOOK_ENV=dev"))
		}
	case "":
		errs = append(errs, errors.New("VAXBOOK_MAILER is required (only \"console\" exists so far, for dev)"))
	default:
		errs = append(errs, fmt.Errorf("VAXBOOK_MAILER %q is not supported", c.Mailer))
	}
	if err := c.LogLevel.UnmarshalText([]byte(orDefault(getenv("VAXBOOK_LOG_LEVEL"), "info"))); err != nil {
		errs = append(errs, fmt.Errorf("VAXBOOK_LOG_LEVEL: %w", err))
	}
	return c, errors.Join(errs...)
}

// Load reads the configuration from the process environment.
func Load() (Config, error) { return FromEnv(os.Getenv) }

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
