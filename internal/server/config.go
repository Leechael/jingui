package server

import (
	"fmt"
	"os"
	"strings"
)

// Config holds server configuration loaded from environment variables.
type Config struct {
	AdminToken  string
	DBPath      string
	ListenAddr  string
	RATLSStrict bool
	CORSOrigins []string
}

// LoadConfig loads server configuration from environment variables.
func LoadConfig() (*Config, error) {
	adminToken := os.Getenv("JINGUI_ADMIN_TOKEN")
	if adminToken == "" {
		return nil, fmt.Errorf("JINGUI_ADMIN_TOKEN is required")
	}
	if len(adminToken) < 16 {
		return nil, fmt.Errorf("JINGUI_ADMIN_TOKEN must be at least 16 characters")
	}

	dbPath := os.Getenv("JINGUI_DB_PATH")
	if dbPath == "" {
		dbPath = "jingui.db"
	}

	listenAddr := os.Getenv("JINGUI_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	ratlsStrict := true
	if v := strings.TrimSpace(strings.ToLower(os.Getenv("JINGUI_RATLS_STRICT"))); v != "" {
		switch v {
		case "1", "true", "yes", "on":
			ratlsStrict = true
		case "0", "false", "no", "off":
			ratlsStrict = false
		default:
			return nil, fmt.Errorf("JINGUI_RATLS_STRICT must be one of true/false/1/0/yes/no/on/off")
		}
	}

	var corsOrigins []string
	if v := os.Getenv("JINGUI_CORS_ORIGINS"); v != "" {
		for _, o := range strings.Split(v, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				corsOrigins = append(corsOrigins, o)
			}
		}
	}

	return &Config{
		AdminToken:  adminToken,
		DBPath:      dbPath,
		ListenAddr:  listenAddr,
		RATLSStrict: ratlsStrict,
		CORSOrigins: corsOrigins,
	}, nil
}
