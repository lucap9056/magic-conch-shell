package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lucap9056/go-envfile/envfile"
	"github.com/lucap9056/go-lifecycle/lifecycle"
	"github.com/lucap9056/magic-conch-shell/middleware/discord-oauth2/internal/auth"
	"github.com/lucap9056/magic-conch-shell/middleware/discord-oauth2/internal/oauth2"
)

var mode = "development"

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func main() {
	envfile.Load()
	life := lifecycle.New()

	databaseUrl := os.Getenv("DATABASE_URL")
	if databaseUrl == "" {
		log.Fatal("Missing required DATA_SOURCE_NAME environment variable")
	}

	db, err := auth.NewDatabase(databaseUrl)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	jwtManager := auth.NewJWTManager(db, 15*time.Minute, 7*24*time.Hour)

	httpAddress := os.Getenv("HTTP_ADDRESS")
	if httpAddress == "" {
		httpAddress = ":80"
	}

	clientID := os.Getenv("DISCORD_CLIENT_ID")
	clientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	redirectURL := os.Getenv("DISCORD_REDIRECT_URL")
	httpMode := os.Getenv("HTTP_MODE")

	if httpMode != "" {
		mode = httpMode
	}

	devMode := (mode == "development")

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		log.Fatal("Missing required Discord OAuth2 environment variables")
	}

	handler := oauth2.NewHandler(clientID, clientSecret, redirectURL)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {

		b := make([]byte, 16)
		rand.Read(b)
		state := hex.EncodeToString(b)

		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			HttpOnly: true,
			Secure:   !devMode,
			MaxAge:   300,
			SameSite: http.SameSiteLaxMode,
		})

		url := handler.AuthURL(state)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	})

	mux.HandleFunc("GET /callback", func(w http.ResponseWriter, r *http.Request) {

		{
			returnedState := r.FormValue("state")

			stateCookie, err := r.Cookie("oauth_state")
			if err != nil {
				sendJSONResponse(w, false, "State cookie missing", http.StatusBadRequest, nil)
				return
			}

			if returnedState == "" || returnedState != stateCookie.Value {
				sendJSONResponse(w, false, "Invalid OAuth state (CSRF detected)", http.StatusForbidden, nil)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:   "oauth_state",
				MaxAge: -1,
				Path:   "/",
			})
		}

		code := r.FormValue("code")
		deviceName := r.Header.Get("X-Device-Name")
		if deviceName == "" {
			deviceName = "Unknown Device"
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		token, err := handler.Exchange(ctx, code)
		if err != nil {
			sendJSONResponse(w, false, "Code exchange failed", http.StatusInternalServerError, err)
			return
		}

		discordUser, err := handler.GetUser(ctx, token)
		if err != nil {
			sendJSONResponse(w, false, "Failed to fetch user info", http.StatusInternalServerError, err)
			return
		}

		user, err := db.GetUserFromEmail(discordUser.Email)
		if err != nil {
			sendJSONResponse(w, false, "Database error", http.StatusInternalServerError, err)
			return
		}

		if user == nil {
			sendJSONResponse(w, false, "User not found", http.StatusNotFound, nil)
			return
		}

		userID := fmt.Sprint(user.UserID)
		secret := jwtManager.GenerateRandomSecret()

		deviceID, err := db.SaveDeviceSecret(userID, deviceName, secret)
		if err != nil {
			sendJSONResponse(w, false, "Failed to register device session", http.StatusInternalServerError, err)
			return
		}

		refreshToken, err := jwtManager.GenerateRefresh(userID, deviceID, secret)
		if err != nil {
			sendJSONResponse(w, false, "Refresh token generation failed", http.StatusInternalServerError, err)
			return
		}

		accessToken, err := jwtManager.GenerateAccess(refreshToken, user.Username)
		if err != nil {
			sendJSONResponse(w, false, "Access token generation failed", http.StatusInternalServerError, err)
			return
		}

		setRefreshCookie(w, refreshToken, devMode)

		sendJSONResponse(w, true, accessToken, http.StatusOK, nil)
	})

	mux.HandleFunc("POST /refresh", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("refresh_token")
		if err != nil {
			sendJSONResponse(w, false, "Missing refresh cookie", http.StatusUnauthorized, nil)
			return
		}

		claims, err := jwtManager.VerifyRefresh(cookie.Value)
		if err != nil {
			sendJSONResponse(w, false, "Invalid session or expired refresh token", http.StatusUnauthorized, err)
			return
		}

		userID := claims.Subject

		user, err := db.GetUserFromID(userID)
		if err != nil {
			sendJSONResponse(w, false, "User not found", http.StatusUnauthorized, err)
			return
		}

		newRefreshToken, err := jwtManager.GenerateRefresh(userID, claims.DeviceID)
		if err != nil {
			sendJSONResponse(w, false, "Failed to rotate refresh token", http.StatusInternalServerError, err)
			return
		}

		accessToken, err := jwtManager.GenerateAccess(newRefreshToken, user.Username)
		if err != nil {
			sendJSONResponse(w, false, "Access token generation failed", http.StatusInternalServerError, err)
			return
		}

		setRefreshCookie(w, newRefreshToken, devMode)
		sendJSONResponse(w, true, accessToken, http.StatusOK, nil)
	})

	mux.HandleFunc("POST /refresh-access", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("refresh_token")
		if err != nil {
			sendJSONResponse(w, false, "Unauthorized", http.StatusUnauthorized, nil)
			return
		}

		claims, err := jwtManager.VerifyRefresh(cookie.Value)
		if err != nil {
			sendJSONResponse(w, false, "Invalid refresh token", http.StatusUnauthorized, err)
			return
		}

		user, err := db.GetUserFromID(claims.Subject)
		if err != nil {
			sendJSONResponse(w, false, "User not found", http.StatusUnauthorized, err)
			return
		}

		accessToken, err := jwtManager.GenerateAccess(cookie.Value, user.Username)
		if err != nil {
			sendJSONResponse(w, false, "Refresh failed", http.StatusUnauthorized, err)
			return
		}

		sendJSONResponse(w, true, accessToken, http.StatusOK, nil)
	})

	mux.HandleFunc("GET /verify", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			sendJSONResponse(w, false, "Missing Bearer token", http.StatusUnauthorized, nil)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwtManager.Verify(tokenStr)
		if err != nil {
			sendJSONResponse(w, false, "Invalid access token", http.StatusUnauthorized, err)
			return
		}

		w.Header().Set("X-Forwarded-User", claims.UserID)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("refresh_token")
		if err == nil && cookie.Value != "" {
			claims, pErr := jwtManager.VerifyRefresh(cookie.Value)
			if pErr == nil {
				err = db.DeleteDevice(claims.Subject, claims.DeviceID)
				if err != nil {
					log.Printf("[WARN] Failed to delete device from DB on logout: %v", err)
				}
			}
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})

		sendJSONResponse(w, true, "Logged out and device session revoked", http.StatusOK, nil)
	})

	server := &http.Server{
		Addr:         httpAddress,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Starting Discord OAuth2 server on %s (Mode: %s)", httpAddress, mode)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			life.Exitln(err.Error())
		}
	}()

	life.OnExit(func() {
		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	life.Wait()
}

func setRefreshCookie(w http.ResponseWriter, token string, devMode bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HttpOnly: true,
		Secure:   !devMode,
		SameSite: http.SameSiteLaxMode,
	})
}

func sendJSONResponse(w http.ResponseWriter, success bool, message string, code int, internalErr error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if internalErr != nil {
		log.Printf("[ERROR] Status: %d, Msg: %s, Err: %v", code, message, internalErr)
	}

	json.NewEncoder(w).Encode(Response{
		Success: success,
		Message: message,
	})
}
