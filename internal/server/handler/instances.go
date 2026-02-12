package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"net/http"

	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

type registerInstanceRequest struct {
	PublicKey   string `json:"public_key" binding:"required"`
	BoundAppID string `json:"bound_app_id" binding:"required"`
	BoundUserID string `json:"bound_user_id" binding:"required"`
	Label       string `json:"label"`
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
			FID:         fid,
			PublicKey:   pubKeyBytes,
			BoundAppID:  req.BoundAppID,
			BoundUserID: req.BoundUserID,
			Label:       req.Label,
		}

		if err := store.RegisterInstance(inst); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "instance already exists or DB error: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"fid": fid, "status": "registered"})
	}
}
