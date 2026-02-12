package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/server/db"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	goauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const oauthStateMaxAge = 10 * time.Minute

// googleCredentials represents the structure inside credentials.json.
type googleCredentials struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURIs []string `json:"redirect_uris"`
}

func parseGoogleCreds(credJSON []byte) (*googleCredentials, error) {
	var wrapper struct {
		Installed *googleCredentials `json:"installed"`
		Web       *googleCredentials `json:"web"`
	}
	if err := json.Unmarshal(credJSON, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}
	if wrapper.Web != nil {
		return wrapper.Web, nil
	}
	if wrapper.Installed != nil {
		return wrapper.Installed, nil
	}
	return nil, fmt.Errorf("no 'installed' or 'web' key in credentials")
}

// makeOAuthState produces an HMAC-signed state: "app_id:timestamp_hex:hmac_hex"
func makeOAuthState(appID string, masterKey [32]byte) string {
	ts := strconv.FormatInt(time.Now().Unix(), 16)
	mac := hmac.New(sha256.New, masterKey[:])
	mac.Write([]byte(appID + ":" + ts))
	sig := hex.EncodeToString(mac.Sum(nil))
	return appID + ":" + ts + ":" + sig
}

// verifyOAuthState verifies and parses the HMAC-signed state, returning the app_id.
func verifyOAuthState(state string, masterKey [32]byte) (string, error) {
	parts := strings.SplitN(state, ":", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed state")
	}
	appID, tsHex, sigHex := parts[0], parts[1], parts[2]

	// Verify HMAC
	mac := hmac.New(sha256.New, masterKey[:])
	mac.Write([]byte(appID + ":" + tsHex))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sigHex), []byte(expectedSig)) {
		return "", fmt.Errorf("invalid state signature")
	}

	// Verify timestamp freshness
	tsUnix, err := strconv.ParseInt(tsHex, 16, 64)
	if err != nil {
		return "", fmt.Errorf("invalid timestamp in state")
	}
	if time.Since(time.Unix(tsUnix, 0)) > oauthStateMaxAge {
		return "", fmt.Errorf("state expired")
	}

	return appID, nil
}

// HandleOAuthGateway handles GET /v1/credentials/gateway/:app_id.
func HandleOAuthGateway(store *db.Store, masterKey [32]byte, baseURL string) gin.HandlerFunc {
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

		credJSON, err := crypto.DecryptAtRest(masterKey, app.CredentialsEncrypted)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt credentials"})
			return
		}

		creds, err := parseGoogleCreds(credJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid stored credentials"})
			return
		}

		callbackURL := baseURL + "/v1/credentials/callback"

		oauthConfig := &oauth2.Config{
			ClientID:     creds.ClientID,
			ClientSecret: creds.ClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  callbackURL,
			Scopes:       []string{"openid", "email"},
		}

		if app.RequiredScopes != "" {
			oauthConfig.Scopes = append(oauthConfig.Scopes, app.RequiredScopes)
		}

		// HMAC-signed state with app_id + timestamp
		state := makeOAuthState(appID, masterKey)
		authURL := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		c.Redirect(http.StatusFound, authURL)
	}
}

// HandleOAuthCallback handles GET /v1/credentials/callback.
func HandleOAuthCallback(store *db.Store, masterKey [32]byte, baseURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")

		if code == "" || state == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing code or state"})
			return
		}

		// Verify HMAC-signed state and extract app_id
		appID, err := verifyOAuthState(state, masterKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired OAuth state: " + err.Error()})
			return
		}

		app, err := store.GetApp(appID)
		if err != nil || app == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid app_id in state"})
			return
		}

		credJSON, err := crypto.DecryptAtRest(masterKey, app.CredentialsEncrypted)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt credentials"})
			return
		}

		creds, err := parseGoogleCreds(credJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid stored credentials"})
			return
		}

		callbackURL := baseURL + "/v1/credentials/callback"

		oauthConfig := &oauth2.Config{
			ClientID:     creds.ClientID,
			ClientSecret: creds.ClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  callbackURL,
			Scopes:       []string{"openid", "email"},
		}

		token, err := oauthConfig.Exchange(context.Background(), code)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "token exchange failed: " + err.Error()})
			return
		}

		if token.RefreshToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no refresh_token returned (try revoking app access and retry)"})
			return
		}

		httpClient := oauthConfig.Client(context.Background(), token)
		oauth2Service, err := goauth2.NewService(context.Background(), option.WithHTTPClient(httpClient))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create oauth2 service"})
			return
		}

		userinfo, err := oauth2Service.Userinfo.Get().Do()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user info: " + err.Error()})
			return
		}

		email := userinfo.Email
		if email == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no email in user info"})
			return
		}

		tokenJSON, _ := json.Marshal(map[string]string{
			"refresh_token": token.RefreshToken,
		})

		encrypted, err := crypto.EncryptAtRest(masterKey, tokenJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
			return
		}

		secret := &db.UserSecret{
			AppID:           appID,
			UserID:          email,
			SecretEncrypted: encrypted,
		}

		if err := store.UpsertUserSecret(secret); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store secret"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "authorized",
			"app_id": appID,
			"email":  email,
		})
	}
}

// HandleDeviceAuth handles POST /v1/credentials/device/:app_id.
// It initiates a Google OAuth Device Authorization Grant (RFC 8628) flow,
// which does not require a redirect URI — suitable for private IP deployments.
func HandleDeviceAuth(store *db.Store, masterKey [32]byte) gin.HandlerFunc {
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

		credJSON, err := crypto.DecryptAtRest(masterKey, app.CredentialsEncrypted)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt credentials"})
			return
		}

		creds, err := parseGoogleCreds(credJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid stored credentials"})
			return
		}

		oauthConfig := &oauth2.Config{
			ClientID:     creds.ClientID,
			ClientSecret: creds.ClientSecret,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"openid", "email"},
		}

		if app.RequiredScopes != "" {
			oauthConfig.Scopes = append(oauthConfig.Scopes, app.RequiredScopes)
		}

		deviceResp, err := oauthConfig.DeviceAuth(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "device auth request failed: " + err.Error()})
			return
		}

		// Poll for token in the background until user authorizes or flow expires.
		go func() {
			ctx, cancel := context.WithDeadline(context.Background(), deviceResp.Expiry)
			defer cancel()

			token, err := oauthConfig.DeviceAccessToken(ctx, deviceResp)
			if err != nil {
				log.Printf("device flow failed: app=%s error=%v", appID, err)
				return
			}

			if token.RefreshToken == "" {
				log.Printf("device flow: no refresh_token returned for app=%s", appID)
				return
			}

			httpClient := oauthConfig.Client(context.Background(), token)
			oauth2Service, err := goauth2.NewService(context.Background(), option.WithHTTPClient(httpClient))
			if err != nil {
				log.Printf("device flow: failed to create oauth2 service: app=%s error=%v", appID, err)
				return
			}

			userinfo, err := oauth2Service.Userinfo.Get().Do()
			if err != nil {
				log.Printf("device flow: failed to get user info: app=%s error=%v", appID, err)
				return
			}

			email := userinfo.Email
			if email == "" {
				log.Printf("device flow: no email in user info: app=%s", appID)
				return
			}

			tokenJSON, _ := json.Marshal(map[string]string{
				"refresh_token": token.RefreshToken,
			})

			encrypted, err := crypto.EncryptAtRest(masterKey, tokenJSON)
			if err != nil {
				log.Printf("device flow: encryption failed: app=%s error=%v", appID, err)
				return
			}

			secret := &db.UserSecret{
				AppID:           appID,
				UserID:          email,
				SecretEncrypted: encrypted,
			}

			if err := store.UpsertUserSecret(secret); err != nil {
				log.Printf("device flow: failed to store secret: app=%s error=%v", appID, err)
				return
			}

			log.Printf("device flow completed: app=%s email=%s", appID, email)
		}()

		c.JSON(http.StatusOK, gin.H{
			"user_code":        deviceResp.UserCode,
			"verification_url": deviceResp.VerificationURI,
			"expires_in":       int(time.Until(deviceResp.Expiry).Seconds()),
		})
	}
}
