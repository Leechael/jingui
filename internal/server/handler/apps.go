package handler

import (
	"encoding/json"
	"net/http"

	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

type createAppRequest struct {
	Vault           string          `json:"vault" binding:"required"`
	Name            string          `json:"name" binding:"required"`
	ServiceType     string          `json:"service_type"`
	RequiredScopes  string          `json:"required_scopes"`
	CredentialsJSON json.RawMessage `json:"credentials_json"`
}

// isEmptyJSONObject returns true if data is a JSON object with no keys.
// Returns false for null, arrays, strings, and other non-object types.
func isEmptyJSONObject(data json.RawMessage) bool {
	// json.Unmarshal decodes null into a nil map; reject that explicitly.
	if len(data) == 0 || string(data) == "null" {
		return false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return false
	}
	return m != nil && len(m) == 0
}

// validateOAuthCredentials checks that credentials_json contains an
// "installed" or "web" key. Call only when credentials are non-empty.
func validateOAuthCredentials(c *gin.Context, data json.RawMessage) bool {
	var creds struct {
		Installed *json.RawMessage `json:"installed"`
		Web       *json.RawMessage `json:"web"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials_json format"})
		return false
	}
	if creds.Installed == nil && creds.Web == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "credentials_json must contain 'installed' or 'web' key"})
		return false
	}
	return true
}

// HandleCreateApp handles POST /v1/apps.
func HandleCreateApp(store *db.Store, masterKey [32]byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createAppRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Default empty credentials to {}
		if len(req.CredentialsJSON) == 0 {
			req.CredentialsJSON = json.RawMessage(`{}`)
		}

		// Validate credentials_json only when non-empty
		if !isEmptyJSONObject(req.CredentialsJSON) {
			if !validateOAuthCredentials(c, req.CredentialsJSON) {
				return
			}
		}

		// Encrypt credentials at rest
		encrypted, err := crypto.EncryptAtRest(masterKey, req.CredentialsJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
			return
		}

		app := &db.App{
			Vault:                req.Vault,
			Name:                 req.Name,
			ServiceType:          req.ServiceType,
			RequiredScopes:       req.RequiredScopes,
			CredentialsEncrypted: encrypted,
		}

		if err := store.CreateApp(app); err != nil {
			if err == db.ErrAppDuplicate {
				c.JSON(http.StatusConflict, gin.H{
					"error": "vault already exists",
					"hint":  "use PUT /v1/apps/:app_id to update vault metadata/credentials",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create app"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"vault": req.Vault, "status": "created"})
	}
}

// HandleUpdateApp handles PUT /v1/apps/:vault.
func HandleUpdateApp(store *db.Store, masterKey [32]byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")

		var req createAppRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Vault != "" && req.Vault != appID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "vault in body must match path"})
			return
		}

		// Look up existing app for preserving credentials
		existing, err := store.GetApp(appID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve app"})
			return
		}
		if existing == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
			return
		}

		// Determine encrypted credentials to use
		var encrypted []byte
		if len(req.CredentialsJSON) == 0 || isEmptyJSONObject(req.CredentialsJSON) {
			// No credentials provided (or explicitly empty) — preserve existing
			encrypted = existing.CredentialsEncrypted
		} else {
			// Non-empty credentials supplied — validate and encrypt
			if !validateOAuthCredentials(c, req.CredentialsJSON) {
				return
			}
			encrypted, err = crypto.EncryptAtRest(masterKey, req.CredentialsJSON)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
				return
			}
		}

		ok, err := store.UpdateApp(&db.App{
			Vault:                appID,
			Name:                 req.Name,
			ServiceType:          req.ServiceType,
			RequiredScopes:       req.RequiredScopes,
			CredentialsEncrypted: encrypted,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update app"})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"vault": appID, "status": "updated"})
	}
}
