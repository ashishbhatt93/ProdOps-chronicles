package pod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prodops-chronicles/prodops/internal/content"
)

// Seeder sends module content to the backend's internal seed endpoint.
type Seeder struct {
	backendURL string
	token      string
	client     *http.Client
}

func NewSeeder(backendURL, token string) *Seeder {
	return &Seeder{
		backendURL: backendURL,
		token:      token,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

// Seed POSTs the module content to the backend. Retries up to maxAttempts.
func (s *Seeder) Seed(moduleID string, mc *content.ModuleContent) error {
	payload := map[string]any{
		"module_id": moduleID,
		"content":   mc,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal seed payload: %w", err)
	}

	const maxAttempts = 5
	const backoff = 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = s.doSeed(body)
		if err == nil {
			return nil
		}
		if attempt < maxAttempts {
			time.Sleep(backoff)
		}
	}
	return fmt.Errorf("seed failed after %d attempts: %w", maxAttempts, err)
}

func (s *Seeder) doSeed(body []byte) error {
	req, err := http.NewRequest(http.MethodPost,
		s.backendURL+"/api/v1/internal/modules/seed",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("backend unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("seed returned %d", resp.StatusCode)
	}
	return nil
}
