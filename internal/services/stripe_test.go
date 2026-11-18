package services

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v87/webhook"
)

func TestIsHTTPSURL(t *testing.T) {
	tests := map[string]bool{
		"https://cdn.cloudflare.steamstatic.com/a.jpg": true,
		"http://example.com/a.jpg":                     false,
		"https://":                                     false,
		"/images/a.jpg":                                false,
		"":                                             false,
		"not a url":                                    false,
	}

	for raw, want := range tests {
		if got := isHTTPSURL(raw); got != want {
			t.Errorf("isHTTPSURL(%q) = %v; want %v", raw, got, want)
		}
	}
}

func TestConstructEvent(t *testing.T) {
	const secret = "whsec_test_secret"
	s := NewStripeService("sk_test_unused", secret)

	payload := []byte(`{"id":"evt_1","object":"event","type":"checkout.session.completed","data":{"object":{"id":"cs_test_1","payment_status":"paid"}}}`)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: time.Now()})

	event, err := s.ConstructEvent(payload, signed.Header)
	if err != nil {
		t.Fatalf("valid signature: %v", err)
	}
	if event.ID != "evt_1" || event.Type != "checkout.session.completed" {
		t.Errorf("event = %s %s; want evt_1 checkout.session.completed", event.ID, event.Type)
	}

	var object struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(event.Object, &object); err != nil || object.ID != "cs_test_1" {
		t.Errorf("object id = %q, %v; want cs_test_1", object.ID, err)
	}

	// The same body signed with another secret must be rejected.
	forged := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: "whsec_other", Timestamp: time.Now()})
	if _, err := s.ConstructEvent(payload, forged.Header); err == nil {
		t.Error("wrong secret: want an error")
	}

	// A changed body must be rejected.
	if _, err := s.ConstructEvent([]byte(`{"id":"evt_2"}`), signed.Header); err == nil {
		t.Error("changed body: want an error")
	}

	// Old signatures are rejected, so a captured request can't be replayed later.
	old := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: time.Now().Add(-time.Hour)})
	if _, err := s.ConstructEvent(payload, old.Header); err == nil {
		t.Error("old timestamp: want an error")
	}

	if _, err := s.ConstructEvent(payload, "t="+strconv.FormatInt(time.Now().Unix(), 10)); err == nil {
		t.Error("missing v1 signature: want an error")
	}
}

func TestConstructEventWithoutSecret(t *testing.T) {
	s := NewStripeService("sk_test_unused", "")

	// Signed with an empty secret: must still be rejected when no secret is configured.
	payload := []byte(`{"id":"evt_1","object":"event","type":"checkout.session.completed"}`)
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: "", Timestamp: time.Now()})

	if _, err := s.ConstructEvent(payload, signed.Header); !errors.Is(err, ErrWebhookNotConfigured) {
		t.Errorf("err = %v; want ErrWebhookNotConfigured", err)
	}
}
