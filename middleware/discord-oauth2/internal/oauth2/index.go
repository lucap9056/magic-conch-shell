package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ravener/discord-oauth2"
	"golang.org/x/oauth2"
)

type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Bot           bool   `json:"bot"`
	System        bool   `json:"system"`
	MFAEnabled    bool   `json:"mfa_enabled"`
	Locale        string `json:"locale"`
	Verified      bool   `json:"verified"`
	Email         string `json:"email"`
	Flags         int    `json:"flags"`
	PremiumType   int    `json:"premium_type"`
	PublicFlags   int    `json:"public_flags"`
}

type Handler struct {
	config *oauth2.Config
}

func NewHandler(clientID, clientSecret, redirectURL string) *Handler {
	return &Handler{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{discord.ScopeIdentify},
			Endpoint:     discord.Endpoint,
		},
	}
}

func (h *Handler) AuthURL(state string) string {
	return h.config.AuthCodeURL(state)
}

func (h *Handler) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return h.config.Exchange(ctx, code)
}

func (h *Handler) GetUser(ctx context.Context, token *oauth2.Token) (*DiscordUser, error) {
	client := h.config.Client(ctx, token)

	resp, err := client.Get("https://discord.com/api/users/@me")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord api returned status: %s", resp.Status)
	}

	var user DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &user, nil
}
