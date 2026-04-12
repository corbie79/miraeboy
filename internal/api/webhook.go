package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/corbie79/miraeboy/internal/storage"
)

func (s *Server) DispatchWebhook(repo, event, pkg, actor string) {
	hooks, err := s.store.GetWebhooksForEvent(repo, event)
	if err != nil || len(hooks) == 0 {
		return
	}
	payload := storage.WebhookEvent{
		ID: newID(), Event: event, Repo: repo,
		Package: pkg, Actor: actor, Timestamp: time.Now().UTC(),
	}
	for _, wh := range hooks {
		go deliverWebhook(wh, payload)
	}
}

func deliverWebhook(wh storage.WebhookConfig, event storage.WebhookEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		return
	}
	delays := []time.Duration{0, 5 * time.Second, 30 * time.Second}
	client := &http.Client{Timeout: 10 * time.Second}
	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}
		req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Miraeboy-Event", event.Event)
		req.Header.Set("X-Miraeboy-Delivery", event.ID)
		if wh.Secret != "" {
			mac := hmac.New(sha256.New, []byte(wh.Secret))
			mac.Write(body)
			req.Header.Set("X-Miraeboy-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[webhook] attempt %d %s→%s: %v", attempt+1, event.Event, wh.URL, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		log.Printf("[webhook] attempt %d %s→%s: HTTP %d", attempt+1, event.Event, wh.URL, resp.StatusCode)
	}
}

type webhookReq struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret"`
	Active *bool    `json:"active"`
}

var validWebhookEvents = map[string]bool{
	"*": true, "package.upload": true, "package.delete": true,
	"cargo.publish": true, "cargo.yank": true, "cargo.unyank": true,
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	repoName := r.PathValue("repository")
	var req webhookReq
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" || (!strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://")) {
		jsonError(w, http.StatusBadRequest, "url must be a valid http/https URL")
		return
	}
	if len(req.Events) == 0 {
		req.Events = []string{"*"}
	}
	for _, e := range req.Events {
		if !validWebhookEvents[e] {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("unknown event: %q", e))
			return
		}
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	wh := storage.WebhookConfig{
		ID: hex.EncodeToString(b), URL: req.URL,
		Events: req.Events, Secret: req.Secret, Active: active,
	}
	if err := s.store.SaveWebhook(repoName, wh); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, wh)
}

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	repoName := r.PathValue("repository")
	hooks, err := s.store.ListWebhooks(repoName)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if hooks == nil {
		hooks = []storage.WebhookConfig{}
	}
	for i := range hooks {
		hooks[i].Secret = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": hooks})
}

func (s *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	repoName, id := r.PathValue("repository"), r.PathValue("id")
	wh, err := s.store.GetWebhook(repoName, id)
	if err != nil || wh == nil {
		jsonError(w, http.StatusNotFound, "webhook not found")
		return
	}
	var req webhookReq
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL != "" {
		wh.URL = req.URL
	}
	if len(req.Events) > 0 {
		wh.Events = req.Events
	}
	if req.Secret != "" {
		wh.Secret = req.Secret
	}
	if req.Active != nil {
		wh.Active = *req.Active
	}
	if err := s.store.SaveWebhook(repoName, *wh); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	wh.Secret = ""
	writeJSON(w, http.StatusOK, wh)
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	repoName, id := r.PathValue("repository"), r.PathValue("id")
	if err := s.store.DeleteWebhook(repoName, id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	repoName, id := r.PathValue("repository"), r.PathValue("id")
	wh, err := s.store.GetWebhook(repoName, id)
	if err != nil || wh == nil {
		jsonError(w, http.StatusNotFound, "webhook not found")
		return
	}
	event := storage.WebhookEvent{
		ID: newID(), Event: "ping", Repo: repoName, Timestamp: time.Now().UTC(),
	}
	go deliverWebhook(*wh, event)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ping sent", "delivery_id": event.ID})
}
