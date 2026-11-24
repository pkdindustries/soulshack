package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/urfave/cli/v3"

	"pkdindustries/soulshack/internal/config"
)

func parseConfig(t *testing.T, args ...string) *config.Configuration {
	t.Helper()
	var got *config.Configuration
	cmd := &cli.Command{
		Name:  "soulshack",
		Flags: config.GetFlags(),
		Action: func(_ context.Context, c *cli.Command) error {
			got = config.NewConfiguration(c)
			return nil
		},
	}
	if err := cmd.Run(context.Background(), append([]string{"soulshack"}, args...)); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSessionDurationDefaultsWithSessionDB(t *testing.T) {
	// Default in-memory mode keeps 10m idle expiry.
	inMem := parseConfig(t)
	if inMem.Session.TTL != 10*time.Minute {
		t.Fatalf("in-memory default TTL = %s, want 10m", inMem.Session.TTL)
	}

	// Persistent --sessiondb defaults to no expiry unless sessionduration is explicitly set.
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	persisted := parseConfig(t, "--sessiondb", dbPath)
	if persisted.Session.TTL != 0 {
		t.Fatalf("persisted default TTL = %s, want 0", persisted.Session.TTL)
	}

	// Explicit --sessionduration flag overrides the persisted default.
	explicitFlag := parseConfig(t, "--sessiondb", dbPath, "--sessionduration", "24h")
	if explicitFlag.Session.TTL != 24*time.Hour {
		t.Fatalf("persisted explicit flag TTL = %s, want 24h", explicitFlag.Session.TTL)
	}

	// Explicit env var also overrides the persisted default.
	t.Setenv("SOULSHACK_SESSIONDURATION", "2h")
	explicitEnv := parseConfig(t, "--sessiondb", dbPath)
	if explicitEnv.Session.TTL != 2*time.Hour {
		t.Fatalf("persisted explicit env TTL = %s, want 2h", explicitEnv.Session.TTL)
	}

	// Explicit YAML value also overrides the persisted default.
	t.Setenv("SOULSHACK_SESSIONDURATION", "")
	yamlPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(yamlPath, []byte("sessiondb: "+dbPath+"\nsessionduration: 30m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOULSHACK_CONFIG", yamlPath)
	explicitYAML := parseConfig(t)
	if explicitYAML.Session.TTL != 30*time.Minute {
		t.Fatalf("persisted explicit YAML TTL = %s, want 30m", explicitYAML.Session.TTL)
	}
}
