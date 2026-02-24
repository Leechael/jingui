package handler

import (
	"encoding/hex"
	"errors"
	"log"
	"net/http"

	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

// instanceView is used to serialize TEE instances with hex-encoded public keys
// instead of the default base64 encoding for []byte.
type instanceView struct {
	FID         string  `json:"fid"`
	PublicKey   string  `json:"public_key"`
	BoundAppID  string  `json:"bound_app_id"`
	BoundUserID string  `json:"bound_user_id"`
	Label       string  `json:"label"`
	CreatedAt   string  `json:"created_at"`
	LastUsedAt  *string `json:"last_used_at"`
}

func newInstanceView(inst *db.TEEInstance) instanceView {
	v := instanceView{
		FID:         inst.FID,
		PublicKey:   hex.EncodeToString(inst.PublicKey),
		BoundAppID:  inst.BoundAppID,
		BoundUserID: inst.BoundUserID,
		Label:       inst.Label,
		CreatedAt:   inst.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if inst.LastUsedAt != nil {
		s := inst.LastUsedAt.Format("2006-01-02T15:04:05Z")
		v.LastUsedAt = &s
	}
	return v
}

// --- Apps ---

// HandleListApps handles GET /v1/apps.
func HandleListApps(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		apps, err := store.ListApps()
		if err != nil {
			log.Printf("ListApps error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list apps"})
			return
		}
		c.JSON(http.StatusOK, apps)
	}
}

// HandleGetApp handles GET /v1/apps/:app_id.
func HandleGetApp(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")
		app, err := store.GetApp(appID)
		if err != nil {
			log.Printf("GetApp(%q) error: %v", appID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve app"})
			return
		}
		if app == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"app_id":          app.AppID,
			"name":            app.Name,
			"service_type":    app.ServiceType,
			"required_scopes": app.RequiredScopes,
			"has_credentials": len(app.CredentialsEncrypted) > 0,
			"created_at":      app.CreatedAt,
		})
	}
}

// HandleDeleteApp handles DELETE /v1/apps/:app_id.
func HandleDeleteApp(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")
		cascade := c.Query("cascade") == "true"

		var deleted bool
		var err error
		if cascade {
			deleted, err = store.DeleteAppCascade(appID)
		} else {
			deleted, err = store.DeleteApp(appID)
		}

		if err != nil {
			if errors.Is(err, db.ErrAppHasDependents) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			} else {
				log.Printf("DeleteApp(%q) error: %v", appID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete app"})
			}
			return
		}
		if !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted", "app_id": appID})
	}
}

// --- Instances ---

// HandleListInstances handles GET /v1/instances.
func HandleListInstances(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		instances, err := store.ListInstances()
		if err != nil {
			log.Printf("ListInstances error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list instances"})
			return
		}
		views := make([]instanceView, len(instances))
		for i := range instances {
			views[i] = newInstanceView(&instances[i])
		}
		c.JSON(http.StatusOK, views)
	}
}

// HandleGetInstance handles GET /v1/instances/:fid.
func HandleGetInstance(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		fid := c.Param("fid")
		inst, err := store.GetInstance(fid)
		if err != nil {
			log.Printf("GetInstance(%q) error: %v", fid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve instance"})
			return
		}
		if inst == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}
		c.JSON(http.StatusOK, newInstanceView(inst))
	}
}

// HandleDeleteInstance handles DELETE /v1/instances/:fid.
func HandleDeleteInstance(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		fid := c.Param("fid")
		deleted, err := store.DeleteInstance(fid)
		if err != nil {
			log.Printf("DeleteInstance(%q) error: %v", fid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete instance"})
			return
		}
		if !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted", "fid": fid})
	}
}

// --- User Secrets ---

// HandleListUserSecrets handles GET /v1/user-secrets.
func HandleListUserSecrets(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Query("app_id")

		var secrets []db.UserSecret
		var err error
		if appID != "" {
			secrets, err = store.ListUserSecretsByApp(appID)
		} else {
			secrets, err = store.ListUserSecrets()
		}

		if err != nil {
			log.Printf("ListUserSecrets error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list user secrets"})
			return
		}
		c.JSON(http.StatusOK, secrets)
	}
}

// HandleGetUserSecret handles GET /v1/user-secrets/:app_id/:user_id.
func HandleGetUserSecret(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")
		userID := c.Param("user_id")
		secret, err := store.GetUserSecret(appID, userID)
		if err != nil {
			log.Printf("GetUserSecret(%q, %q) error: %v", appID, userID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user secret"})
			return
		}
		if secret == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user secret not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"app_id":     secret.AppID,
			"user_id":    secret.UserID,
			"has_secret": len(secret.SecretEncrypted) > 0,
			"created_at": secret.CreatedAt,
			"updated_at": secret.UpdatedAt,
		})
	}
}

// HandleDeleteUserSecret handles DELETE /v1/user-secrets/:app_id/:user_id.
func HandleDeleteUserSecret(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")
		userID := c.Param("user_id")
		cascade := c.Query("cascade") == "true"

		var deleted bool
		var err error
		if cascade {
			deleted, err = store.DeleteUserSecretCascade(appID, userID)
		} else {
			deleted, err = store.DeleteUserSecret(appID, userID)
		}

		if err != nil {
			if errors.Is(err, db.ErrSecretHasDependents) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			} else {
				log.Printf("DeleteUserSecret(%q, %q) error: %v", appID, userID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user secret"})
			}
			return
		}
		if !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "user secret not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted", "app_id": appID, "user_id": userID})
	}
}
