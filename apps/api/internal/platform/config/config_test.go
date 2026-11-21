package config_test

import (
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/apps/api/internal/platform/config"
)

func env(overrides map[string]string) func(string) string {
	base := map[string]string{
		"VAXBOOK_ENV":          "dev",
		"VAXBOOK_DATABASE_URL": "postgres://localhost/vaxbook",
		"VAXBOOK_APP_HOST":     "localhost",
		"VAXBOOK_OTP_PEPPER":   strings.Repeat("p", config.MinPepperLen),
		"VAXBOOK_MAILER":       "console",
	}
	for k, v := range overrides {
		base[k] = v
	}
	return func(k string) string { return base[k] }
}

func TestDevConfig(t *testing.T) {
	c, err := config.FromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.Env != config.EnvDev {
		t.Fatalf("defaults wrong: %+v", c)
	}
}

func TestConsoleMailerIsDevOnly(t *testing.T) {
	for _, e := range []string{"prod", ""} { // empty defaults to prod
		_, err := config.FromEnv(env(map[string]string{"VAXBOOK_ENV": e}))
		if err == nil || !strings.Contains(err.Error(), "console") {
			t.Errorf("VAXBOOK_ENV=%q: err = %v, want console mailer rejected", e, err)
		}
	}
}

func TestInvalidConfig(t *testing.T) {
	cases := map[string]map[string]string{
		"no database":   {"VAXBOOK_DATABASE_URL": ""},
		"no host":       {"VAXBOOK_APP_HOST": ""},
		"host w/ port":  {"VAXBOOK_APP_HOST": "localhost:8080"},
		"short pepper":  {"VAXBOOK_OTP_PEPPER": "short"},
		"no mailer":     {"VAXBOOK_MAILER": ""},
		"bad env":       {"VAXBOOK_ENV": "staging"},
		"bad log level": {"VAXBOOK_LOG_LEVEL": "loud"},
	}
	for name, o := range cases {
		if _, err := config.FromEnv(env(o)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
