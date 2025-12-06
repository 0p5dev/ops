package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/urfave/cli/v3"
)

type AuthTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
	User         *UserInfo `json:"user,omitempty"`
}

type UserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type SupabaseAuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

func Login(ctx context.Context, cmd *cli.Command) error {
	supabaseURL := cmd.Metadata["supabaseUrl"].(string)
	supabaseKey := cmd.Metadata["supabaseKey"].(string)

	if supabaseURL == "" || supabaseKey == "" {
		return fmt.Errorf("supabase URL and Key must be configured. Set SUPABASE_URL and SUPABASE_KEY environment variables or add them to ~/.config/ops/config.json")
	}

	// Store supabaseKey for use in callbacks
	supabaseAnonKey := supabaseKey

	// Create a channel to receive the auth response
	responseChan := make(chan *SupabaseAuthResponse, 1)
	errChan := make(chan error, 1)

	// Start local server to handle OAuth callback
	server := &http.Server{Addr: ":54321"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Supabase redirects with hash fragment, we need to parse it from the URL
		// The frontend would normally handle this, but we'll extract from query params
		accessToken := r.URL.Query().Get("access_token")
		refreshToken := r.URL.Query().Get("refresh_token")
		expiresIn := r.URL.Query().Get("expires_in")

		if accessToken == "" {
			// Try to get from hash fragment (this won't work directly in Go, need JS redirect)
			fmt.Fprintf(w, `<html><body><script>
				const hash = window.location.hash.substring(1);
				const params = new URLSearchParams(hash);
				const accessToken = params.get('access_token');
				const refreshToken = params.get('refresh_token');
				const expiresIn = params.get('expires_in');
				if (accessToken) {
					window.location.href = '/token?access_token=' + accessToken + 
						'&refresh_token=' + refreshToken + 
						'&expires_in=' + expiresIn;
				} else {
					document.body.innerHTML = '<h1>Authentication failed</h1><p>No access token received.</p>';
				}
			</script></body></html>`)
			return
		}

		// Parse the response
		var expiresInInt int64
		fmt.Sscanf(expiresIn, "%d", &expiresInInt)

		// Get user info
		userInfo, err := getUserInfo(supabaseURL, supabaseAnonKey, accessToken)
		if err != nil {
			errChan <- fmt.Errorf("failed to get user info: %w", err)
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1><p>Failed to retrieve user information.</p></body></html>")
			return
		}

		response := &SupabaseAuthResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    expiresInInt,
			TokenType:    "bearer",
			User:         *userInfo,
		}

		responseChan <- response
		fmt.Fprintf(w, "<html><body><h1>Authentication successful!</h1><p>You can close this window and return to the terminal.</p></body></html>")
	})

	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		accessToken := r.URL.Query().Get("access_token")
		refreshToken := r.URL.Query().Get("refresh_token")
		expiresIn := r.URL.Query().Get("expires_in")

		if accessToken == "" {
			errChan <- fmt.Errorf("no access token received")
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1><p>No access token received.</p></body></html>")
			return
		}

		var expiresInInt int64
		fmt.Sscanf(expiresIn, "%d", &expiresInInt)

		// Get user info
		userInfo, err := getUserInfo(supabaseURL, supabaseAnonKey, accessToken)
		if err != nil {
			errChan <- fmt.Errorf("failed to get user info: %w", err)
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1><p>Failed to retrieve user information.</p></body></html>")
			return
		}

		response := &SupabaseAuthResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    expiresInInt,
			TokenType:    "bearer",
			User:         *userInfo,
		}

		responseChan <- response
		fmt.Fprintf(w, "<html><body><h1>Authentication successful!</h1><p>You can close this window and return to the terminal.</p></body></html>")
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Generate OAuth URL for GitHub
	redirectURL := "http://localhost:54321/callback"
	authURL := fmt.Sprintf("%s/auth/v1/authorize?provider=github&redirect_to=%s", supabaseURL, url.QueryEscape(redirectURL))

	fmt.Println("Please visit this URL to authenticate with GitHub:")
	fmt.Println(authURL)

	// Wait for callback or timeout
	select {
	case response := <-responseChan:
		// Shutdown the server
		go server.Shutdown(context.Background())

		// Store tokens
		tokens := AuthTokens{
			AccessToken:  response.AccessToken,
			RefreshToken: response.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
			TokenType:    response.TokenType,
			User: &UserInfo{
				ID:    response.User.ID,
				Email: response.User.Email,
			},
		}

		if err := saveTokens(tokens); err != nil {
			return fmt.Errorf("failed to save tokens: %w", err)
		}

		fmt.Println("Login successful! Credentials saved.")
		if tokens.User != nil {
			fmt.Printf("Logged in as: %s\n", tokens.User.Email)
		}

		return nil

	case err := <-errChan:
		server.Shutdown(context.Background())
		return fmt.Errorf("authentication error: %w", err)

	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		return fmt.Errorf("authentication timeout: no response received within 5 minutes")
	}
}

func getUserInfo(supabaseURL, supabaseKey, accessToken string) (*struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/auth/v1/user", supabaseURL), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("apikey", supabaseKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: %s", string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

func saveTokens(tokens AuthTokens) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".config", "ops")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	credentialsFile := filepath.Join(configDir, "credentials.json")
	tokenJSON, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(credentialsFile, tokenJSON, 0600)
}

func GetBearerToken() (string, error) {
	tokens, err := loadTokens()
	if err != nil {
		return "", err
	}

	// Check if token is expired or about to expire (within 5 minutes)
	if time.Now().Add(5 * time.Minute).After(tokens.ExpiresAt) {
		// Token is expired or expiring soon, should refresh
		// For now, just return an error asking user to login again
		return "", fmt.Errorf("authentication token expired, run 'ops login' to reauthenticate")
	}

	return tokens.AccessToken, nil
}

func loadTokens() (*AuthTokens, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	credentialsFile := filepath.Join(homeDir, ".config", "ops", "credentials.json")
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("credentials not found, run 'ops login' to authenticate")
	}

	credentialsData, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, err
	}

	var tokens AuthTokens
	if err := json.Unmarshal(credentialsData, &tokens); err != nil {
		return nil, err
	}

	return &tokens, nil
}
