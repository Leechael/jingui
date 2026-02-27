package logx

import (
	"os"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"", false},
		{"bad", true},
	}
	for _, c := range cases {
		_, err := ParseLevel(c.in)
		if c.wantErr && err == nil {
			t.Fatalf("expected error for %q", c.in)
		}
		if !c.wantErr && err != nil {
			t.Fatalf("unexpected error for %q: %v", c.in, err)
		}
	}
}

func TestConfigurePrecedence(t *testing.T) {
	t.Setenv("JINGUI_LOG_LEVEL", "warn")
	if err := Configure("", false); err != nil {
		t.Fatalf("configure env: %v", err)
	}
	if IsDebug() {
		t.Fatalf("expected non-debug from env warn")
	}

	if err := Configure("", true); err != nil {
		t.Fatalf("configure verbose: %v", err)
	}
	if !IsDebug() {
		t.Fatalf("expected debug from verbose")
	}

	if err := Configure("error", true); err != nil {
		t.Fatalf("configure explicit: %v", err)
	}
	if IsDebug() {
		t.Fatalf("expected non-debug from explicit error")
	}

	_ = os.Unsetenv("JINGUI_LOG_LEVEL")
}
