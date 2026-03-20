package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	discordAuthorizeURL = "https://discord.com/api/oauth2/authorize"
	discordTokenURL     = "https://discord.com/api/oauth2/token"
	discordAPIBase      = "https://discord.com/api/v10"
	oauthScopes         = "identify guilds"
)

// DiscordUser holds the user info returned by Discord's /users/@me endpoint.
type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// DiscordGuild holds partial guild info from /users/@me/guilds.
type DiscordGuild struct {
	ID string `json:"id"`
}

// OAuthConfig holds the Discord OAuth2 application configuration.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// GenerateState creates a random state parameter for CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// AuthorizeURL builds the Discord OAuth2 authorization URL.
func (c *OAuthConfig) AuthorizeURL(state string) string {
	params := url.Values{
		"client_id":     {c.ClientID},
		"redirect_uri":  {c.RedirectURL},
		"response_type": {"code"},
		"scope":         {oauthScopes},
		"state":         {state},
	}
	return discordAuthorizeURL + "?" + params.Encode()
}

// tokenResponse is the JSON response from Discord's token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// ExchangeCode exchanges an authorization code for an access token.
func (c *OAuthConfig) ExchangeCode(code string) (string, error) {
	data := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.RedirectURL},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(discordTokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	return tok.AccessToken, nil
}

// FetchUser retrieves the authenticated user's profile from Discord.
func FetchUser(accessToken string) (*DiscordUser, error) {
	user, err := discordGet[DiscordUser](accessToken, "/users/@me")
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FetchUserGuilds retrieves the list of guilds the authenticated user belongs to.
func FetchUserGuilds(accessToken string) ([]DiscordGuild, error) {
	return discordGet[[]DiscordGuild](accessToken, "/users/@me/guilds")
}

// IsGuildMember checks whether the user belongs to the specified guild.
func IsGuildMember(accessToken, guildID string) (bool, error) {
	guilds, err := FetchUserGuilds(accessToken)
	if err != nil {
		return false, err
	}
	for _, g := range guilds {
		if g.ID == guildID {
			return true, nil
		}
	}
	return false, nil
}

// discordGet performs an authenticated GET to the Discord API and decodes JSON.
func discordGet[T any](accessToken, path string) (T, error) {
	var zero T

	req, err := http.NewRequest(http.MethodGet, discordAPIBase+path, nil)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("discord API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return zero, fmt.Errorf("discord API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("decode discord response: %w", err)
	}

	return result, nil
}
