package server

import (
	"github.com/aspect-build/jingui/internal/attestation"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/aspect-build/jingui/internal/server/handler"
	"github.com/gin-gonic/gin"
)

// NewRouter creates and configures the Gin router with all routes.
func NewRouter(store *db.Store, cfg *Config) *gin.Engine {
	r := gin.Default()

	if len(cfg.CORSOrigins) > 0 {
		r.Use(CORS(cfg.CORSOrigins))
	}

	r.GET("/", func(c *gin.Context) {
		c.String(200, "ok")
	})
	r.StaticFile("/openapi.json", "docs/openapi.json")

	admin := AdminAuth(cfg.AdminToken)
	verifier := attestation.NewRATLSVerifier()
	collector := attestation.NewDstackInfoCollector("")

	v1 := r.Group("/v1")
	{
		// Vaults
		v1.POST("/vaults", admin, handler.HandleCreateVault(store))
		v1.GET("/vaults", admin, handler.HandleListVaults(store))
		v1.GET("/vaults/:id", admin, handler.HandleGetVault(store))
		v1.PUT("/vaults/:id", admin, handler.HandleUpdateVault(store))
		v1.DELETE("/vaults/:id", admin, handler.HandleDeleteVault(store))

		// Vault items
		v1.GET("/vaults/:id/items", admin, handler.HandleListItems(store))
		v1.GET("/vaults/:id/items/:section", admin, handler.HandleGetItem(store))
		v1.PUT("/vaults/:id/items/:section", admin, handler.HandlePutItem(store))
		v1.DELETE("/vaults/:id/items/:section", admin, handler.HandleDeleteItem(store))

		// Vault ↔ Instance access
		v1.GET("/vaults/:id/instances", admin, handler.HandleListVaultInstances(store))
		v1.POST("/vaults/:id/instances/:fid", admin, handler.HandleGrantVaultAccess(store))
		v1.DELETE("/vaults/:id/instances/:fid", admin, handler.HandleRevokeVaultAccess(store))

		// Instances
		v1.POST("/instances", admin, handler.HandleRegisterInstance(store))
		v1.GET("/instances", admin, handler.HandleListInstances(store))
		v1.GET("/instances/:fid", admin, handler.HandleGetInstance(store))
		v1.PUT("/instances/:fid", admin, handler.HandleUpdateInstance(store))
		v1.DELETE("/instances/:fid", admin, handler.HandleDeleteInstance(store))

		// Debug policy
		v1.GET("/debug-policy/:vault/:fid", admin, handler.HandleGetDebugPolicy(store))
		v1.PUT("/debug-policy/:vault/:fid", admin, handler.HandlePutDebugPolicy(store))

		// Client proof-of-possession challenge (no admin auth).
		v1.POST("/secrets/challenge", handler.HandleIssueChallenge(store, cfg.RATLSStrict, verifier, collector))

		// Secret fetch — requires proof-of-possession challenge response, then returns
		// payload encrypted to the registered TEE public key.
		v1.POST("/secrets/fetch", handler.HandleFetchSecrets(store, cfg.RATLSStrict))
	}

	return r
}
