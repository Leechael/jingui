package handler

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"

	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

// instanceView is used to serialize TEE instances with hex-encoded public keys
// instead of the default base64 encoding for []byte.
type instanceView struct {
	FID                   string  `json:"fid"`
	PublicKey             string  `json:"public_key"`
	BoundVault            string  `json:"bound_vault"`
	BoundAttestationAppID string  `json:"bound_attestation_app_id"`
	BoundItem             string  `json:"bound_item"`
	Label                 string  `json:"label"`
	CreatedAt             string  `json:"created_at"`
	LastUsedAt            *string `json:"last_used_at"`
}

func newInstanceView(inst *db.TEEInstance) instanceView {
	v := instanceView{
		FID:                   inst.FID,
		PublicKey:             hex.EncodeToString(inst.PublicKey),
		BoundVault:            inst.BoundVault,
		BoundAttestationAppID: inst.BoundAttestationAppID,
		BoundItem:             inst.BoundItem,
		Label:                 inst.Label,
		CreatedAt:             inst.CreatedAt.Format("2006-01-02T15:04:05Z"),
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
		vault := c.Param("app_id")
		app, err := store.GetApp(vault)
		if err != nil {
			log.Printf("GetApp(%q) error: %v", vault, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve app"})
			return
		}
		if app == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"vault":           app.Vault,
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
		vault := c.Param("app_id")
		cascade := c.Query("cascade") == "true"

		var deleted bool
		var err error
		if cascade {
			deleted, err = store.DeleteAppCascade(vault)
		} else {
			deleted, err = store.DeleteApp(vault)
		}

		if err != nil {
			if errors.Is(err, db.ErrAppHasDependents) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			} else {
				log.Printf("DeleteApp(%q) error: %v", vault, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete app"})
			}
			return
		}
		if !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "app not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted", "vault": vault})
	}
}

// --- Instances ---

// HandleListInstances handles GET /v1/instances.
// Accepts optional ?vault=X query param to filter by bound vault.
func HandleListInstances(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Query("vault")

		var instances []db.TEEInstance
		var err error
		if vault != "" {
			instances, err = store.ListInstancesByVault(vault)
		} else {
			instances, err = store.ListInstances()
		}
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

// --- Secrets (Vault Items) ---

// HandleListSecrets handles GET /v1/secrets.
func HandleListSecrets(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Query("vault")

		var items []db.VaultItem
		var err error
		if vault != "" {
			items, err = store.ListVaultItemsByVault(vault)
		} else {
			items, err = store.ListVaultItems()
		}

		if err != nil {
			log.Printf("ListSecrets error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list secrets"})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

// decryptSecretKeys decrypts the encrypted secret blob and returns the sorted key names.
func decryptSecretKeys(masterKey [32]byte, encrypted []byte) []string {
	if len(encrypted) == 0 {
		return nil
	}
	plaintext, err := crypto.DecryptAtRest(masterKey, encrypted)
	if err != nil {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(plaintext, &m); err != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// HandleGetSecret handles GET /v1/secrets/:vault/:item.
func HandleGetSecret(store *db.Store, masterKey [32]byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Param("vault")
		item := c.Param("item")
		secret, err := store.GetVaultItem(vault, item)
		if err != nil {
			log.Printf("GetSecret(%q, %q) error: %v", vault, item, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve secret"})
			return
		}
		if secret == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "secret not found"})
			return
		}

		resp := gin.H{
			"vault":      secret.Vault,
			"item":       secret.Item,
			"has_secret": len(secret.SecretEncrypted) > 0,
			"created_at": secret.CreatedAt,
			"updated_at": secret.UpdatedAt,
		}
		if keys := decryptSecretKeys(masterKey, secret.SecretEncrypted); keys != nil {
			resp["secret_keys"] = keys
		}
		c.JSON(http.StatusOK, resp)
	}
}

// HandleGetSecretData handles GET /v1/secrets/:vault/:item/data.
// Returns the full decrypted key-value pairs for a secret.
func HandleGetSecretData(store *db.Store, masterKey [32]byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Param("vault")
		item := c.Param("item")
		secret, err := store.GetVaultItem(vault, item)
		if err != nil {
			log.Printf("GetSecretData(%q, %q) error: %v", vault, item, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve secret"})
			return
		}
		if secret == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "secret not found"})
			return
		}
		if len(secret.SecretEncrypted) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"vault":       vault,
				"item":        item,
				"secret_keys": []string{},
				"data":        map[string]string{},
			})
			return
		}

		plaintext, err := crypto.DecryptAtRest(masterKey, secret.SecretEncrypted)
		if err != nil {
			log.Printf("GetSecretData(%q, %q) decrypt error: %v", vault, item, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt secret"})
			return
		}

		var data map[string]string
		if err := json.Unmarshal(plaintext, &data); err != nil {
			log.Printf("GetSecretData(%q, %q) unmarshal error: %v", vault, item, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse secret data"})
			return
		}

		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		c.JSON(http.StatusOK, gin.H{
			"vault":       vault,
			"item":        item,
			"secret_keys": keys,
			"data":        data,
		})
	}
}

// HandleDeleteSecret handles DELETE /v1/secrets/:vault/:item.
func HandleDeleteSecret(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Param("vault")
		item := c.Param("item")
		cascade := c.Query("cascade") == "true"

		var deleted bool
		var err error
		if cascade {
			deleted, err = store.DeleteVaultItemCascade(vault, item)
		} else {
			deleted, err = store.DeleteVaultItem(vault, item)
		}

		if err != nil {
			if errors.Is(err, db.ErrItemHasDependents) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			} else {
				log.Printf("DeleteSecret(%q, %q) error: %v", vault, item, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete secret"})
			}
			return
		}
		if !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "secret not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted", "vault": vault, "item": item})
	}
}

// HandleGetDebugPolicy handles GET /v1/debug-policy/:vault/:item.
func HandleGetDebugPolicy(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Param("vault")
		item := c.Param("item")
		p, err := store.GetDebugPolicy(vault, item)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get debug policy"})
			return
		}
		if p == nil {
			// default allow=true if no policy row exists
			c.JSON(http.StatusOK, gin.H{"vault": vault, "item": item, "allow_read_debug": true, "source": "default"})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

type putDebugPolicyRequest struct {
	AllowReadDebug bool `json:"allow_read_debug"`
}

// HandlePutDebugPolicy handles PUT /v1/debug-policy/:vault/:item.
func HandlePutDebugPolicy(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Param("vault")
		item := c.Param("item")
		var req putDebugPolicyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpsertDebugPolicy(vault, item, req.AllowReadDebug); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update debug policy"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated", "vault": vault, "item": item, "allow_read_debug": req.AllowReadDebug})
	}
}
