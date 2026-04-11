package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
	RedirectBaseURL    string
	WebAppURL          string
	HTTPClient         *http.Client
}

type OAuthUser struct {
	Email    string
	Name     string
	Provider string
}

func ExchangeGoogleCode(cfg OAuthConfig, code string) (OAuthUser, error) {
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	data := url.Values{
		"code":          {code},
		"client_id":     {cfg.GoogleClientID},
		"client_secret": {cfg.GoogleClientSecret},
		"redirect_uri":  {cfg.RedirectBaseURL + "/api/v1/auth/google/callback"},
		"grant_type":    {"authorization_code"},
	}

	req, _ := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return OAuthUser{}, fmt.Errorf("exchanging code: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return OAuthUser{}, fmt.Errorf("parsing token response: %w", err)
	}

	userResp, err := httpGetWithAuth(client, "https://www.googleapis.com/oauth2/v2/userinfo", tokenResp.AccessToken)
	if err != nil {
		return OAuthUser{}, err
	}
	defer userResp.Body.Close()

	var googleUser struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&googleUser); err != nil {
		return OAuthUser{}, fmt.Errorf("parsing user info: %w", err)
	}

	return OAuthUser{Email: googleUser.Email, Name: googleUser.Name, Provider: "google"}, nil
}

func ExchangeGitHubCode(cfg OAuthConfig, code string) (OAuthUser, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {cfg.GitHubClientID},
		"client_secret": {cfg.GitHubClientSecret},
	}

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return OAuthUser{}, fmt.Errorf("exchanging code: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return OAuthUser{}, fmt.Errorf("parsing token response: %w", err)
	}

	userResp, err := httpGetWithAuth(client, "https://api.github.com/user", tokenResp.AccessToken)
	if err != nil {
		return OAuthUser{}, err
	}
	defer userResp.Body.Close()

	var ghUser struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Login string `json:"login"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&ghUser); err != nil {
		return OAuthUser{}, fmt.Errorf("parsing user info: %w", err)
	}

	if ghUser.Email == "" {
		ghUser.Email, _ = fetchGitHubPrimaryEmail(client, tokenResp.AccessToken)
	}

	if ghUser.Email == "" {
		return OAuthUser{}, fmt.Errorf("could not get email from GitHub")
	}

	name := ghUser.Name
	if name == "" {
		name = ghUser.Login
	}

	return OAuthUser{Email: ghUser.Email, Name: name, Provider: "github"}, nil
}

func fetchGitHubPrimaryEmail(client *http.Client, token string) (string, error) {
	resp, err := httpGetWithAuth(client, "https://api.github.com/user/emails", token)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no primary email")
}

func httpGetWithAuth(client *http.Client, url, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("request to %s returned %d: %s", url, resp.StatusCode, string(body))
	}

	return resp, nil
}
