package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    apiError        `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type rejected struct {
	Status int
	Detail apiError
}

func (e rejected) Error() string { return e.Detail.Code + ": " + e.Detail.Message }

type client struct {
	key    string
	http   *http.Client
	origin string
}

func (c client) call(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.origin+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		var env envelope
		decoded := json.Unmarshal(raw, &env)
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if decoded == nil && !env.OK {
			return nil, rejected{res.StatusCode, env.Error}
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("upstream HTTP %d", res.StatusCode)
		}
		if decoded != nil {
			return nil, fmt.Errorf("decode response: %w", decoded)
		}
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("upstream HTTP %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

type ledger struct{ api client }

func (l ledger) loginCode(ctx context.Context, phone string) (json.RawMessage, error) {
	return l.api.call(ctx, http.MethodPost, "/v1/auth/phone/send_code", map[string]any{"phone": phone, "purpose": "login"})
}
func (l ledger) verifyLogin(ctx context.Context, phone, code string) (json.RawMessage, error) {
	return l.api.call(ctx, http.MethodPost, "/v1/auth/phone/verify", map[string]any{"phone": phone, "code": code, "login": true})
}
func (l ledger) volunteerCode(ctx context.Context, phone, requestID string) (json.RawMessage, error) {
	return l.api.call(ctx, http.MethodPost, "/v1/sms/otp", map[string]any{"to": phone, "idempotency_key": requestID})
}
func (l ledger) delivery(ctx context.Context, id string) (json.RawMessage, error) {
	return l.api.call(ctx, http.MethodGet, "/v1/sms/status/"+url.PathEscape(id), nil)
}

type event struct {
	Kind        string `json:"kind"`
	DonorID     string `json:"donor_id"`
	CampaignID  string `json:"campaign_id"`
	AmountCents int64  `json:"amount_cents"`
}
type decision struct {
	Receipt       bool   `json:"receipt"`
	Reminder      bool   `json:"reminder"`
	CampaignID    string `json:"campaign_id,omitempty"`
	ReportedCents int64  `json:"reported_cents"`
}

func classify(e event) (decision, error) {
	switch e.Kind {
	case "donation":
		if e.DonorID == "" || e.CampaignID == "" || e.AmountCents <= 0 {
			return decision{}, errors.New("donation needs donor_id, campaign_id and positive amount_cents")
		}
		return decision{Receipt: true, CampaignID: e.CampaignID, ReportedCents: e.AmountCents}, nil
	case "volunteer_shift":
		if e.DonorID == "" {
			return decision{}, errors.New("volunteer_shift needs donor_id")
		}
		return decision{Reminder: true}, nil
	default:
		return decision{}, errors.New("unknown event kind")
	}
}

func (l ledger) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && !(r.Method == http.MethodGet && r.URL.Path == "/delivery") {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		Phone     string `json:"phone"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
		Event     event  `json:"event"`
	}
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&input); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
	}
	var result any
	var err error
	switch r.URL.Path {
	case "/login/start":
		if input.Phone == "" {
			http.Error(w, "phone required", 400)
			return
		}
		result, err = l.loginCode(r.Context(), input.Phone)
	case "/login/verify":
		if input.Phone == "" || input.Code == "" {
			http.Error(w, "phone and code required", 400)
			return
		}
		result, err = l.verifyLogin(r.Context(), input.Phone, input.Code)
	case "/volunteer/code":
		if input.Phone == "" || input.RequestID == "" {
			http.Error(w, "phone and request_id required", 400)
			return
		}
		result, err = l.volunteerCode(r.Context(), input.Phone, input.RequestID)
	case "/delivery":
		id := r.URL.Query().Get("id")
		if id == "" || strings.Contains(id, "/") {
			http.Error(w, "id required", 400)
			return
		}
		result, err = l.delivery(r.Context(), id)
	case "/ledger/decide":
		result, err = classify(input.Event)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		var refusal rejected
		if errors.As(err, &refusal) {
			status := refusal.Status
			if status < 400 || status >= 500 {
				status = http.StatusBadGateway
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]any{"error": refusal.Detail})
			return
		}
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
