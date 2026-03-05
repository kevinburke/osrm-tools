// Package osrm provides an HTTP client for the OSRM routing engine.
package osrm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kevinburke/osrm-tools/geo"
)

// CriticalError indicates a response parsing failure that likely means the OSRM server
// is incompatible or misconfigured. Callers should treat this as fatal.
type CriticalError struct {
	Err error
}

func (e *CriticalError) Error() string {
	return e.Err.Error()
}

func (e *CriticalError) Unwrap() error {
	return e.Err
}

// Client is an OSRM HTTP client.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Logger     *slog.Logger
}

// NewClient creates a new OSRM client with the given base URL (e.g. "http://localhost:9367").
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		Logger: slog.Default(),
	}
}

// GetRoute queries OSRM for a route between two points using the given profile.
func (c *Client) GetRoute(ctx context.Context, profile string, from, to geo.Point, options map[string]string) (*RouteResponse, error) {
	routeURL := fmt.Sprintf("%s/route/v1/%s/%.6f,%.6f;%.6f,%.6f",
		c.BaseURL, profile, from.Lon, from.Lat, to.Lon, to.Lat)

	params := url.Values{}
	for key, value := range options {
		params.Add(key, value)
	}

	fullURL := routeURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query OSRM: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSRM returned status %d: %s", resp.StatusCode, truncateForLog(body))
	}

	var osrmResp RouteResponse
	if err := json.Unmarshal(body, &osrmResp); err != nil {
		return nil, &CriticalError{Err: fmt.Errorf("failed to parse OSRM route response JSON (this may indicate an incompatible OSRM server version): %w", err)}
	}

	return &osrmResp, nil
}

// GetRouteWithGeometry queries OSRM for a cycling route with full geometry, retrying
// without alternatives if the server cannot provide them.
func (c *Client) GetRouteWithGeometry(ctx context.Context, profile string, from, to geo.Point) (*RouteResponse, error) {
	options := map[string]string{
		"overview":     "full",
		"geometries":   "geojson",
		"steps":        "true",
		"annotations":  "true",
		"alternatives": "3",
	}

	osrmResp, err := c.GetRoute(ctx, profile, from, to, options)
	if err != nil {
		return nil, err
	}

	if osrmResp.Code == "Ok" && len(osrmResp.Routes) > 0 {
		return osrmResp, nil
	}

	c.Logger.Warn("OSRM alternatives request failed, retrying without alternatives",
		"code", osrmResp.Code, "message", osrmResp.Message)

	delete(options, "alternatives")
	osrmResp, err = c.GetRoute(ctx, profile, from, to, options)
	if err != nil {
		return nil, err
	}

	if osrmResp.Code != "Ok" || len(osrmResp.Routes) == 0 {
		return nil, fmt.Errorf("no route found (code=%s message=%s)", osrmResp.Code, osrmResp.Message)
	}

	return osrmResp, nil
}

// GetNearest queries OSRM's nearest service for roads near a point.
func (c *Client) GetNearest(ctx context.Context, profile string, point geo.Point, number int) (*NearestResponse, error) {
	nearestURL := fmt.Sprintf("%s/nearest/v1/%s/%.6f,%.6f",
		c.BaseURL, profile, point.Lon, point.Lat)

	params := url.Values{}
	params.Add("number", fmt.Sprintf("%d", number))

	fullURL := nearestURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query OSRM nearest: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read nearest response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSRM nearest returned status %d: %s", resp.StatusCode, truncateForLog(body))
	}

	var nearestResp NearestResponse
	if err := json.Unmarshal(body, &nearestResp); err != nil {
		return nil, &CriticalError{Err: fmt.Errorf("failed to parse OSRM nearest response JSON (this may indicate an incompatible OSRM server version): %w", err)}
	}

	return &nearestResp, nil
}

func truncateForLog(body []byte) string {
	summary := strings.TrimSpace(string(body))
	if len(summary) > 200 {
		return summary[:200] + "..."
	}
	return summary
}
