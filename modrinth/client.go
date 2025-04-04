package modrinth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"modrinth-mod-updater/config" // Use correct module path
	"go.uber.org/zap"
)

const (
	modrinthAPIURL = "https://api.modrinth.com/v2"
	defaultTimeout = 5 * time.Second
)

// Client handles communication with the Modrinth API.
type Client struct {
	BaseURL    string
	APIKey     string
	UserAgent  string
	HTTPClient *http.Client
}

// NewClient creates a new Modrinth API client using the provided configuration.
func NewClient(cfg config.Config) (*Client, error) {
	// API Key is required for follow-related actions, validation can happen here or before calling NewClient
	// if cfg.ModrinthAPIKey == "" {
	// 	return nil, fmt.Errorf("MODRINTH_API_KEY is not configured")
	// }
	if cfg.UserAgent == "" {
		// Should be handled by LoadConfig default, but double-check
		return nil, fmt.Errorf("USERAGENT is not configured")
	}

	return &Client{
		BaseURL:   modrinthAPIURL,
		APIKey:    cfg.ModrinthAPIKey,
		UserAgent: cfg.UserAgent,
		HTTPClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

func (c *Client) makeRequest(method, path string, queryParams url.Values, target interface{}, requiresAuth bool, isBinary bool) (*http.Response, error) {
	fullURL := c.BaseURL + path
	if isBinary {
		// For binary downloads, the 'path' is expected to be the full URL already
		fullURL = path
	}

	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if queryParams != nil {
		req.URL.RawQuery = queryParams.Encode()
	}

	req.Header.Set("User-Agent", c.UserAgent)
	if requiresAuth {
		if c.APIKey == "" {
			return nil, fmt.Errorf("authentication required, but MODRINTH_API_KEY is not set")
		}
		req.Header.Set("Authorization", c.APIKey)
	}

	if !isBinary {
		req.Header.Set("Accept", "application/json")
	} else {
		req.Header.Set("Accept", "application/octet-stream")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to read body for more error info, but don't fail if it's already closed or unreadable
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close() // Close body even on error
		return resp, fmt.Errorf("api request failed: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Don't try to decode JSON or close body for binary responses here
	if target != nil && !isBinary {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return resp, fmt.Errorf("failed to decode json response: %w", err)
		}
	}

	return resp, nil // For binary, return the response so the caller can handle the body
}

func (c *Client) GetFollowedProjects() ([]Project, error) {

	var user User
	_, err := c.makeRequest("GET", "/user", nil, &user, true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	if user.ID == "" {
		return nil, fmt.Errorf("could not determine user ID from API key")
	}

	var projects []Project
	_, err = c.makeRequest("GET", fmt.Sprintf("/user/%s/follows", user.ID), nil, &projects, true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get followed projects: %w", err)
	}
	return projects, nil
}

// GetProjectVersions retrieves versions for a specific project, filtered by game version and loader.
func (c *Client) GetProjectVersions(slug, gameVersion, loader string) ([]Version, error) {
	params := url.Values{}
	// Construct JSON array strings manually to avoid Sprintf issues
	params.Add("game_versions", "[\""+gameVersion+"\"]")
	params.Add("loaders", "[\""+loader+"\"]")

	var versions []Version
	_, err := c.makeRequest("GET", fmt.Sprintf("/project/%s/version", slug), params, &versions, true, false) // Assuming auth might be needed based on Python client
	if err != nil {
		return nil, fmt.Errorf("failed to get project versions for '%s': %w", slug, err)
	}
	return versions, nil
}

// GetProject retrieves details for a specific project.
func (c *Client) GetProject(slug string) (*Project, error) {
	var project Project
	_, err := c.makeRequest("GET", fmt.Sprintf("/project/%s", slug), nil, &project, true, false) // Assuming auth might be needed
	if err != nil {
		return nil, fmt.Errorf("failed to get project '%s': %w", slug, err)
	}
	return &project, nil
}

// DownloadModFile downloads a mod file from the given URL and saves it to the specified filename within the 'mods' directory.
func (c *Client) DownloadModFile(log *zap.SugaredLogger, filename, downloadURL string) error {
	modsDir := "mods"
	if err := os.MkdirAll(modsDir, 0755); err != nil {
		return fmt.Errorf("failed to create mods directory '%s': %w", modsDir, err)
	}

	filePath := filepath.Join(modsDir, filename)

	resp, err := c.makeRequest("GET", downloadURL, nil, nil, false, true) // No auth needed for direct download URL, binary=true
	if err != nil {
		return fmt.Errorf("failed to start download for '%s' from %s: %w", filename, downloadURL, err)
	}
	defer resp.Body.Close()

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file '%s': %w", filePath, err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		// Attempt to remove partially downloaded file on error
		os.Remove(filePath)
		return fmt.Errorf("failed to write downloaded content to '%s': %w", filePath, err)
	}

	return nil
}

// --- Structs for API Responses (Basic Definitions) ---
// These should be expanded based on actual API response structure.

// User represents a Modrinth user (simplified).
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	// Add other fields as needed
}

// Project represents a Modrinth project
type Project struct {
	Slug        string `json:"slug"`
	ID          string `json:"id"`         // Add Modrinth Project ID
	Title       string `json:"title"`
	IconURL     string `json:"icon_url"` // Add Icon URL
	Color       int    `json:"color"`    // Add Color (integer representation)
	Updated     string `json:"updated"`  // Add Last Updated Timestamp (string for simplicity)
	ProjectType string `json:"project_type"` // e.g., "mod"
	// Add other fields as needed (description, etc.)
}

// Version represents a Modrinth project version (simplified).
type Version struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	VersionNumber string `json:"version_number"`
	Files         []File `json:"files"`
	// Add other fields (changelog, dependencies, game_versions, loaders, etc.)
}

// File represents a file within a Modrinth version (simplified).
type File struct {
	Filename string            `json:"filename"`
	URL      string            `json:"url"`
	Primary  bool              `json:"primary"`
	Size     int               `json:"size"`
	Hashes   map[string]string `json:"hashes"` // e.g., {"sha512": "...", "sha1": "..."}
	// Add other fields
}
