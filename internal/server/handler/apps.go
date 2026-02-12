package handler

import (
	"encoding/json"
	"net/http"

	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

type createAppRequest struct {
	AppID           string          `json:"app_id" binding:"required"`
	Name            string          `json:"name" binding:"required"`
	ServiceType     string          `json:"service_type" binding:"required"`
	RequiredScopes  string          `json:"required_scopes"`
	CredentialsJSON json.RawMessage `json:"credentials_json" binding:"required"`
}

// HandleCreateApp handles POST /v1/apps.
func HandleCreateApp(store *db.Store, masterKey [32]byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createAppRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate credentials_json is valid JSON with expected fields
		var creds struct {
			Installed *json.RawMessage `json:"installed"`
			Web       *json.RawMessage `json:"web"`
		}
		if err := json.Unmarshal(req.CredentialsJSON, &creds); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials_json format"})
			return
		}
		if creds.Installed == nil && creds.Web == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "credentials_json must contain 'installed' or 'web' key"})
			return
		}

		// Encrypt credentials at rest
		encrypted, err := crypto.EncryptAtRest(masterKey, req.CredentialsJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
			return
		}

		app := &db.App{
			AppID:                req.AppID,
			Name:                 req.Name,
			ServiceType:          req.ServiceType,
			RequiredScopes:       req.RequiredScopes,
			CredentialsEncrypted: encrypted,
		}

		if err := store.CreateApp(app); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "app already exists or DB error: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"app_id": req.AppID, "status": "created"})
	}
}
