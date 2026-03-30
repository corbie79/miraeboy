package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// WebhookConfig defines a webhook endpoint for a repository.
type WebhookConfig struct {
	ID     string   `json:"id"`
	URL    string   `json:"url"`              // HTTPS endpoint to POST events to
	Events []string `json:"events"`           // e.g. ["package.upload","cargo.publish","package.delete"]
	Secret string   `json:"secret,omitempty"` // HMAC-SHA256 secret (write-only; omitted from list responses)
	Active bool     `json:"active"`
}

// WebhookEvent is the payload POSTed to the webhook URL.
type WebhookEvent struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`             // "package.upload", "cargo.publish", etc.
	Repo      string    `json:"repo"`
	Package   string    `json:"package,omitempty"` // "name/version@ns/ch" or "name/version"
	Actor     string    `json:"actor,omitempty"`   // username who triggered the event
	Timestamp time.Time `json:"timestamp"`
}

func webhookKey(repo, id string) string { return fmt.Sprintf("_webhooks/%s/%s.json", repo, id) }
func webhookPrefix(repo string) string  { return fmt.Sprintf("_webhooks/%s/", repo) }

func (s *Storage) GetWebhook(repo, id string) (*WebhookConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.b.Get(webhookKey(repo, id))
	if err != nil {
		return nil, nil
	}
	var wh WebhookConfig
	if err := json.Unmarshal(data, &wh); err != nil {
		return nil, err
	}
	return &wh, nil
}

func (s *Storage) ListWebhooks(repo string) ([]WebhookConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys, err := s.b.List(webhookPrefix(repo))
	if err != nil {
		return []WebhookConfig{}, nil
	}
	var hooks []WebhookConfig
	for _, key := range keys {
		if !strings.HasSuffix(key, ".json") {
			continue
		}
		data, err := s.b.Get(key)
		if err != nil {
			continue
		}
		var wh WebhookConfig
		if err := json.Unmarshal(data, &wh); err != nil {
			continue
		}
		hooks = append(hooks, wh)
	}
	return hooks, nil
}

func (s *Storage) SaveWebhook(repo string, wh WebhookConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return putJSON(s.b, webhookKey(repo, wh.ID), wh)
}

func (s *Storage) DeleteWebhook(repo, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Delete(webhookKey(repo, id))
}

// GetWebhooksForEvent returns all active webhooks for a repo that are subscribed to the given event.
func (s *Storage) GetWebhooksForEvent(repo, event string) ([]WebhookConfig, error) {
	all, err := s.ListWebhooks(repo)
	if err != nil {
		return nil, err
	}
	var matched []WebhookConfig
	for _, wh := range all {
		if !wh.Active {
			continue
		}
		for _, e := range wh.Events {
			if e == event || e == "*" {
				matched = append(matched, wh)
				break
			}
		}
	}
	return matched, nil
}
