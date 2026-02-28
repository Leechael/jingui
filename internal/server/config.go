package server

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
)

// Config holds server configuration loaded from environment variables.
type Config struct {
	MasterKey   [32]byte
	AdminToken  string
	DBPath      string
	ListenAddr  string
	BaseURL     string
	RATLSStrict bool
	CORSOrigins []string
}

// LoadConfig loads server configuration from environment variables.
func LoadConfig() (*Config, error) {
	masterKeyHex := os.Getenv("JINGUI_MASTER_KEY")
	if masterKeyHex == "" {
		return nil, fmt.Errorf("JINGUI_MASTER_KEY is required")
	}
	masterKeyHex = strings.TrimSpace(masterKeyHex)
	if len(masterKeyHex) != 64 {
		preview := masterKeyHex
		if len(preview) > 8 {
			preview = preview[:4] + "..." + preview[len(preview)-4:]
		}
		return nil, fmt.Errorf("JINGUI_MASTER_KEY must be 64 hex characters (32 bytes), got %d chars: %s", len(masterKeyHex), preview)
	}
	mkBytes, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("JINGUI_MASTER_KEY invalid hex: %w", err)
	}
	var masterKey [32]byte
	copy(masterKey[:], mkBytes)

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

	baseURL := os.Getenv("JINGUI_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost" + listenAddr
	}

	if !strings.HasPrefix(baseURL, "https://") {
		log.Printf("WARNING: JINGUI_BASE_URL is not HTTPS (%s). Use HTTPS in production.", baseURL)
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
		MasterKey:   masterKey,
		AdminToken:  adminToken,
		DBPath:      dbPath,
		ListenAddr:  listenAddr,
		BaseURL:     baseURL,
		RATLSStrict: ratlsStrict,
		CORSOrigins: corsOrigins,
	}, nil
}
