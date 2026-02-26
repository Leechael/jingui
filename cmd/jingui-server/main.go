package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aspect-build/jingui/internal/logx"
	"github.com/aspect-build/jingui/internal/server"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/aspect-build/jingui/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	verbose := flag.Bool("verbose", false, "Enable verbose debug logs (same as --log-level debug)")
	logLevel := flag.String("log-level", "", "Log level: debug|info|warn|error (or JINGUI_LOG_LEVEL)")
	flag.BoolVar(showVersion, "v", false, "Print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", version.String("jingui-server"))
		fmt.Fprintf(os.Stderr, "Jingui server stores OAuth app credentials and serves encrypted secrets to TEE instances.\n\n")
		fmt.Fprintf(os.Stderr, "Environment variables:\n")
		fmt.Fprintf(os.Stderr, "  JINGUI_MASTER_KEY   Master encryption key (64 hex chars, required)\n")
		fmt.Fprintf(os.Stderr, "  JINGUI_ADMIN_TOKEN  Admin Bearer token for management APIs (min 16 chars, required)\n")
		fmt.Fprintf(os.Stderr, "  JINGUI_DB_PATH      SQLite database path (default: jingui.db)\n")
		fmt.Fprintf(os.Stderr, "  JINGUI_LISTEN_ADDR  Listen address (default: :8080)\n")
		fmt.Fprintf(os.Stderr, "  JINGUI_BASE_URL     Public base URL for OAuth callbacks (default: http://localhost:<port>)\n")
		fmt.Fprintf(os.Stderr, "  JINGUI_RATLS_STRICT Enforce strict RA-TLS mode for secret fetch flow (default: true)\n")
		fmt.Fprintf(os.Stderr, "  JINGUI_LOG_LEVEL   Log level for server logs: debug|info|warn|error (default: info)\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String("jingui-server"))
		os.Exit(0)
	}

	if err := logx.Configure(*logLevel, *verbose); err != nil {
		log.Fatalf("configure logging: %v", err)
	}

	cfg, err := server.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store, err := db.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	r := server.NewRouter(store, cfg)
	logx.Infof("server config: ratls_strict=%v base_url=%s", cfg.RATLSStrict, cfg.BaseURL)

	log.Printf("jingui-server listening on %s", cfg.ListenAddr)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
