package server

import (
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/aspect-build/jingui/internal/server/handler"
	"github.com/gin-gonic/gin"
)

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(store *db.Store, cfg *Config) *gin.Engine {
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.String(200, "ok")
	})
	r.StaticFile("/openapi.json", "docs/openapi.json")

	admin := AdminAuth(cfg.AdminToken)

	v1 := r.Group("/v1")
	{
		// Admin-authenticated management endpoints
		v1.POST("/apps", admin, handler.HandleCreateApp(store, cfg.MasterKey))
		v1.PUT("/apps/:app_id", admin, handler.HandleUpdateApp(store, cfg.MasterKey))
		v1.GET("/apps", admin, handler.HandleListApps(store))
		v1.GET("/apps/:app_id", admin, handler.HandleGetApp(store))
		v1.DELETE("/apps/:app_id", admin, handler.HandleDeleteApp(store))

		v1.POST("/instances", admin, handler.HandleRegisterInstance(store))
		v1.GET("/instances", admin, handler.HandleListInstances(store))
		v1.GET("/instances/:fid", admin, handler.HandleGetInstance(store))
		v1.DELETE("/instances/:fid", admin, handler.HandleDeleteInstance(store))

		v1.GET("/user-secrets", admin, handler.HandleListUserSecrets(store))
		v1.GET("/user-secrets/:app_id/:user_id", admin, handler.HandleGetUserSecret(store))
		v1.DELETE("/user-secrets/:app_id/:user_id", admin, handler.HandleDeleteUserSecret(store))

		v1.GET("/debug-policy/:app_id/:user_id", admin, handler.HandleGetDebugPolicy(store))
		v1.PUT("/debug-policy/:app_id/:user_id", admin, handler.HandlePutDebugPolicy(store))

		v1.GET("/credentials/gateway/:app_id", admin, handler.HandleOAuthGateway(store, cfg.MasterKey, cfg.BaseURL))
		v1.POST("/credentials/device/:app_id", admin, handler.HandleDeviceAuth(store, cfg.MasterKey))
		v1.PUT("/credentials/:app_id", admin, handler.HandlePutCredentials(store, cfg.MasterKey))

		// OAuth callback — no admin auth (Google redirects the user's browser here)
		v1.GET("/credentials/callback", handler.HandleOAuthCallback(store, cfg.MasterKey, cfg.BaseURL))

		// Client proof-of-possession challenge (no admin auth).
		v1.POST("/secrets/challenge", handler.HandleIssueChallenge(store))

		// Secret fetch — requires proof-of-possession challenge response, then returns
		// payload encrypted to the registered TEE public key.
		v1.POST("/secrets/fetch", handler.HandleFetchSecrets(store, cfg.MasterKey))
	}

	return r
}
