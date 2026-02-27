package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

type registerInstanceRequest struct {
	PublicKey             string `json:"public_key" binding:"required"`
	BoundVault            string `json:"bound_vault" binding:"required"`
	BoundAttestationAppID string `json:"bound_attestation_app_id" binding:"required"`
	BoundItem             string `json:"bound_item" binding:"required"`
	Label                 string `json:"label"`
}

// HandleRegisterInstance handles POST /v1/instances.
func HandleRegisterInstance(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req registerInstanceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		pubKeyBytes, err := hex.DecodeString(req.PublicKey)
		if err != nil || len(pubKeyBytes) != 32 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "public_key must be 64 hex characters (32 bytes)"})
			return
		}

		// Compute FID = hex(SHA1(pubkey_bytes))
		h := sha1.Sum(pubKeyBytes)
		fid := hex.EncodeToString(h[:])

		inst := &db.TEEInstance{
			FID:                   fid,
			PublicKey:             pubKeyBytes,
			BoundVault:            req.BoundVault,
			BoundAttestationAppID: req.BoundAttestationAppID,
			BoundItem:             req.BoundItem,
			Label:                 req.Label,
		}

		if err := store.RegisterInstance(inst); err != nil {
			switch err {
			case db.ErrInstanceDuplicateFID:
				c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("instance with FID %s already exists", fid)})
			case db.ErrInstanceDuplicateKey:
				c.JSON(http.StatusConflict, gin.H{"error": "another instance with this public key already exists"})
			case db.ErrInstanceAppUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("vault %q with authorized item %q not found; register the vault and complete OAuth authorization first", req.BoundVault, req.BoundItem),
				})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			}
			return
		}

		c.JSON(http.StatusCreated, gin.H{"fid": fid, "status": "registered"})
	}
}

type updateInstanceRequest struct {
	BoundAttestationAppID string `json:"bound_attestation_app_id" binding:"required"`
	Label                 string `json:"label"`
}

// HandleUpdateInstance handles PUT /v1/instances/:fid.
func HandleUpdateInstance(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		fid := c.Param("fid")
		var req updateInstanceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		updated, err := store.UpdateInstance(fid, req.BoundAttestationAppID, req.Label)
		if err != nil {
			log.Printf("UpdateInstance(%q) error: %v", fid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if !updated {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "updated", "fid": fid})
	}
}
