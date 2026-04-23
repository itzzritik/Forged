package api

import (
	"net/http"
	"net/url"

	serverauth "github.com/itzzritik/forged/server/internal/auth"
)

func (s *Server) handleDevAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := readJSON(r, &req); err != nil || req.Email == "" {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}

	user, err := s.DB.UpsertOAuthUser(r.Context(), req.Email, "Dev User", "dev", "dev")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	pair, err := issueTokenPairResponse(r.Context(), s, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate token pair")
		return
	}

	response := map[string]any{
		"user_id": user.ID,
		"email":   user.Email,
		"name":    user.Name,
	}
	for key, value := range pair {
		response[key] = value
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGoogleRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := url.QueryEscape(code)

	authURL := "https://accounts.google.com/o/oauth2/v2/auth" +
		"?client_id=" + s.OAuth.GoogleClientID +
		"&redirect_uri=" + url.QueryEscape(s.OAuth.RedirectBaseURL+"/api/v1/auth/google/callback") +
		"&response_type=code" +
		"&scope=openid+email+profile" +
		"&state=" + state
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	authCode := r.URL.Query().Get("code")
	sessionCode, _ := url.QueryUnescape(r.URL.Query().Get("state"))

	if authCode == "" {
		s.redirectError(w, r, sessionCode, "missing authorization code")
		return
	}

	oauthUser, err := serverauth.ExchangeGoogleCode(s.OAuth, authCode)
	if err != nil {
		s.redirectError(w, r, sessionCode, "google auth failed")
		return
	}

	s.completeOAuth(w, r, oauthUser, sessionCode)
}

func (s *Server) handleGitHubRedirect(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := url.QueryEscape(code)

	authURL := "https://github.com/login/oauth/authorize" +
		"?client_id=" + s.OAuth.GitHubClientID +
		"&scope=user:email" +
		"&state=" + state
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	authCode := r.URL.Query().Get("code")
	sessionCode, _ := url.QueryUnescape(r.URL.Query().Get("state"))

	if authCode == "" {
		s.redirectError(w, r, sessionCode, "missing authorization code")
		return
	}

	oauthUser, err := serverauth.ExchangeGitHubCode(s.OAuth, authCode)
	if err != nil {
		s.redirectError(w, r, sessionCode, "github auth failed")
		return
	}

	s.completeOAuth(w, r, oauthUser, sessionCode)
}

func (s *Server) completeOAuth(w http.ResponseWriter, r *http.Request, oauthUser serverauth.OAuthUser, sessionCode string) {
	user, err := s.DB.UpsertOAuthUser(r.Context(), oauthUser.Email, oauthUser.Name, oauthUser.Provider, "")
	if err != nil {
		s.redirectError(w, r, sessionCode, "could not create account")
		return
	}

	pair, err := issueTokenPairResponse(r.Context(), s, user)
	if err != nil {
		s.redirectError(w, r, sessionCode, "could not generate token pair")
		return
	}

	q := url.Values{
		"access_token":       {pair["access_token"].(string)},
		"access_expires_at":  {pair["access_expires_at"].(string)},
		"refresh_token":      {pair["refresh_token"].(string)},
		"refresh_expires_at": {pair["refresh_expires_at"].(string)},
		"user_id":            {user.ID},
		"email":              {user.Email},
		"name":               {user.Name},
	}

	if sessionCode != "" {
		session, err := s.DB.GetAuthSession(r.Context(), sessionCode)
		if err == nil && session != nil && session.CodeChallenge != nil && session.ChallengeMethod != nil {
			if err := s.DB.ApproveAuthSession(r.Context(), sessionCode, user.ID); err == nil {
				q.Set("code", sessionCode)
			}
		} else {
			if err := s.DB.CompleteAuthSession(r.Context(), sessionCode, pair["access_token"].(string), user.ID, user.Email); err == nil {
				q.Set("code", sessionCode)
			}
		}
	}

	http.Redirect(w, r, s.OAuth.WebAppURL+"/api/auth/callback?"+q.Encode(), http.StatusTemporaryRedirect)
}

func (s *Server) redirectError(w http.ResponseWriter, r *http.Request, sessionCode, msg string) {
	if sessionCode != "" {
		s.DB.FailAuthSession(r.Context(), sessionCode, msg)
		http.Redirect(w, r, s.OAuth.WebAppURL+"/auth/success?error="+url.QueryEscape(msg), http.StatusTemporaryRedirect)
		return
	}
	http.Redirect(w, r, s.OAuth.WebAppURL+"/login?error="+url.QueryEscape(msg), http.StatusTemporaryRedirect)
}
