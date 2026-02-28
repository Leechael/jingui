package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
)

type createVaultRequest struct {
	ID   string `json:"id" binding:"required"`
	Name string `json:"name" binding:"required"`
}

// HandleCreateVault handles POST /v1/vaults.
func HandleCreateVault(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createVaultRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		v := &db.Vault{
			ID:   req.ID,
			Name: req.Name,
		}

		if err := store.CreateVault(v); err != nil {
			if err == db.ErrVaultDuplicate {
				c.JSON(http.StatusConflict, gin.H{
					"error": "vault already exists",
					"hint":  "use PUT /v1/vaults/:id to update",
				})
				return
			}
			log.Printf("CreateVault error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create vault"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"id": req.ID, "status": "created"})
	}
}

// HandleGetVault handles GET /v1/vaults/:id.
func HandleGetVault(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		v, err := store.GetVault(id)
		if err != nil {
			log.Printf("GetVault(%q) error: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve vault"})
			return
		}
		if v == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "vault not found"})
			return
		}
		c.JSON(http.StatusOK, v)
	}
}

type updateVaultRequest struct {
	Name string `json:"name" binding:"required"`
}

// HandleUpdateVault handles PUT /v1/vaults/:id.
func HandleUpdateVault(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req updateVaultRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ok, err := store.UpdateVault(&db.Vault{ID: id, Name: req.Name})
		if err != nil {
			log.Printf("UpdateVault(%q) error: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update vault"})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "vault not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id, "status": "updated"})
	}
}

// HandleListVaults handles GET /v1/vaults.
func HandleListVaults(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		vaults, err := store.ListVaults()
		if err != nil {
			log.Printf("ListVaults error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list vaults"})
			return
		}
		if vaults == nil {
			vaults = []db.Vault{}
		}
		c.JSON(http.StatusOK, vaults)
	}
}

// HandleDeleteVault handles DELETE /v1/vaults/:id.
func HandleDeleteVault(store *db.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cascade := c.Query("cascade") == "true"

		var deleted bool
		var err error
		if cascade {
			deleted, err = store.DeleteVaultCascade(id)
		} else {
			deleted, err = store.DeleteVault(id)
		}

		if err != nil {
			if errors.Is(err, db.ErrVaultHasDependents) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			} else {
				log.Printf("DeleteVault(%q) error: %v", id, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete vault"})
			}
			return
		}
		if !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "vault not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": id})
	}
}
