package handler

import (
	"encoding/json"
	"net/http"

	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

type putCredentialsRequest struct {
	UserID  string            `json:"user_id" binding:"required"`
	Secrets map[string]string `json:"secrets" binding:"required"`
}

// HandlePutCredentials handles PUT /v1/credentials/:app_id.
// Directly stores user secrets for a given app, bypassing OAuth.
func HandlePutCredentials(store *db.Store, masterKey [32]byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")

		app, err := store.GetApp(appID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		if app == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
			return
		}

		var req putCredentialsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		secretJSON, err := json.Marshal(req.Secrets)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal secrets"})
			return
		}

		encrypted, err := crypto.EncryptAtRest(masterKey, secretJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
			return
		}

		secret := &db.UserSecret{
			Vault:           appID,
			UserID:          req.UserID,
			SecretEncrypted: encrypted,
		}

		if err := store.UpsertUserSecret(secret); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store secret"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "stored",
			"app_id":  appID,
			"user_id": req.UserID,
		})
	}
}
