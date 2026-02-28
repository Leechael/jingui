package handler

import (
	"encoding/hex"
	"log"
	"net/http"

	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

// instanceView serializes TEE instances with hex-encoded public keys.
type instanceView struct {
	FID         string  `json:"fid"`
	PublicKey   string  `json:"public_key"`
	DstackAppID string  `json:"dstack_app_id"`
	Label       string  `json:"label"`
	CreatedAt   string  `json:"created_at"`
	LastUsedAt  *string `json:"last_used_at"`
}

func newInstanceView(inst *db.TEEInstance) instanceView {
	v := instanceView{
		FID:         inst.FID,
		PublicKey:   hex.EncodeToString(inst.PublicKey),
		DstackAppID: inst.DstackAppID,
		Label:       inst.Label,
		CreatedAt:   inst.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if inst.LastUsedAt != nil {
		s := inst.LastUsedAt.Format("2006-01-02T15:04:05Z")
		v.LastUsedAt = &s
	}
	return v
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

// --- Vault Items ---

// HandleListItems handles GET /v1/vaults/:id/items — list distinct sections.
func HandleListItems(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vaultID := c.Param("id")
		sections, err := store.ListSections(vaultID)
		if err != nil {
			log.Printf("ListSections(%q) error: %v", vaultID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list items"})
			return
		}
		if sections == nil {
			sections = []string{}
		}
		c.JSON(http.StatusOK, sections)
	}
}

// HandleGetItem handles GET /v1/vaults/:id/items/:section — all fields for section.
func HandleGetItem(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vaultID := c.Param("id")
		section := c.Param("section")
		items, err := store.GetItemFields(vaultID, section)
		if err != nil {
			log.Printf("GetItemFields(%q, %q) error: %v", vaultID, section, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get item"})
			return
		}
		if items == nil {
			items = []db.VaultItem{}
		}

		keys := make([]string, len(items))
		for i, item := range items {
			keys[i] = item.ItemName
		}

		c.JSON(http.StatusOK, gin.H{
			"vault_id": vaultID,
			"section":  section,
			"keys":     keys,
		})
	}
}

type putItemRequest struct {
	Fields map[string]string `json:"fields"`
	Delete []string          `json:"delete"`
}

// HandlePutItem handles PUT /v1/vaults/:id/items/:section — merge upsert/delete fields.
func HandlePutItem(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vaultID := c.Param("id")
		section := c.Param("section")

		var req putItemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(req.Fields) == 0 && len(req.Delete) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one of fields or delete must be provided"})
			return
		}

		if err := store.MergeItemFields(vaultID, section, req.Fields, req.Delete); err != nil {
			log.Printf("MergeItemFields(%q, %q) error: %v", vaultID, section, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save item"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "updated"})
	}
}

// HandleDeleteItem handles DELETE /v1/vaults/:id/items/:section.
func HandleDeleteItem(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vaultID := c.Param("id")
		section := c.Param("section")

		deleted, err := store.DeleteSection(vaultID, section)
		if err != nil {
			log.Printf("DeleteSection(%q, %q) error: %v", vaultID, section, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete item"})
			return
		}
		if !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	}
}

// --- Vault ↔ Instance Access ---

// HandleListVaultInstances handles GET /v1/vaults/:id/instances.
func HandleListVaultInstances(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vaultID := c.Param("id")
		instances, err := store.ListVaultInstances(vaultID)
		if err != nil {
			log.Printf("ListVaultInstances(%q) error: %v", vaultID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list vault instances"})
			return
		}
		views := make([]instanceView, len(instances))
		for i := range instances {
			views[i] = newInstanceView(&instances[i])
		}
		c.JSON(http.StatusOK, views)
	}
}

// HandleGrantVaultAccess handles POST /v1/vaults/:id/instances/:fid.
func HandleGrantVaultAccess(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vaultID := c.Param("id")
		fid := c.Param("fid")

		if err := store.GrantVaultAccess(vaultID, fid); err != nil {
			log.Printf("GrantVaultAccess(%q, %q) error: %v", vaultID, fid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to grant access"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "granted"})
	}
}

// HandleRevokeVaultAccess handles DELETE /v1/vaults/:id/instances/:fid.
func HandleRevokeVaultAccess(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vaultID := c.Param("id")
		fid := c.Param("fid")

		deleted, err := store.RevokeVaultAccess(vaultID, fid)
		if err != nil {
			log.Printf("RevokeVaultAccess(%q, %q) error: %v", vaultID, fid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke access"})
			return
		}
		if !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "access entry not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "revoked"})
	}
}

// --- Debug Policy ---

// HandleGetDebugPolicy handles GET /v1/debug-policy/:vault/:fid.
func HandleGetDebugPolicy(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Param("vault")
		fid := c.Param("fid")
		p, err := store.GetDebugPolicy(vault, fid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get debug policy"})
			return
		}
		if p == nil {
			// default allow=true if no policy row exists
			c.JSON(http.StatusOK, gin.H{"vault_id": vault, "fid": fid, "allow_read": true, "source": "default"})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

type putDebugPolicyRequest struct {
	AllowRead bool `json:"allow_read"`
}

// HandlePutDebugPolicy handles PUT /v1/debug-policy/:vault/:fid.
func HandlePutDebugPolicy(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vault := c.Param("vault")
		fid := c.Param("fid")
		var req putDebugPolicyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.UpsertDebugPolicy(vault, fid, req.AllowRead); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update debug policy"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated", "vault_id": vault, "fid": fid, "allow_read": req.AllowRead})
	}
}
