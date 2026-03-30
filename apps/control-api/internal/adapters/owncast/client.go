package owncast

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (client *Client) Status(ctx context.Context) (adapters.OwncastStatus, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/", nil)
	if err != nil {
		return adapters.OwncastStatus{}, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return adapters.OwncastStatus{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return adapters.OwncastStatus{}, fmt.Errorf("owncast returned %s", response.Status)
	}

	return adapters.OwncastStatus{
		Connected: true,
		Detail:    "Owncast HTTP endpoint reachable",
	}, nil
}
