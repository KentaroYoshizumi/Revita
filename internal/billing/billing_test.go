package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

const testWebhookSecret = "whsec_test_secret"

// signPayload reproduces Stripe's webhook signing scheme
// (https://docs.stripe.com/webhooks#verify-manually) so tests can send
// a payload HandleWebhook will accept.
func signPayload(t *testing.T, payload []byte, secret string, timestamp int64) string {
	t.Helper()
	signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sig)
}

type fakeStore struct {
	upserted     []SubscriptionRecord
	statusCalls  []statusCall
	updateStatus error
}

type statusCall struct {
	stripeSubscriptionID string
	status               string
	currentPeriodEnd     time.Time
}

func (f *fakeStore) Upsert(rec SubscriptionRecord) error {
	f.upserted = append(f.upserted, rec)
	return nil
}

func (f *fakeStore) UpdateStatus(stripeSubscriptionID, status string, currentPeriodEnd time.Time) error {
	f.statusCalls = append(f.statusCalls, statusCall{stripeSubscriptionID, status, currentPeriodEnd})
	return f.updateStatus
}

func newTestClient(store SubscriptionStore) *Client {
	return &Client{WebhookSecret: testWebhookSecret, Store: store}
}

func TestHandleWebhook_CheckoutSessionCompleted(t *testing.T) {
	store := &fakeStore{}
	c := newTestClient(store)

	payload := []byte(`{
		"id": "evt_1",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"customer": "cus_123",
				"subscription": "sub_123",
				"client_reference_id": "user-abc"
			}
		}
	}`)
	sig := signPayload(t, payload, testWebhookSecret, time.Now().Unix())

	if err := c.HandleWebhook(payload, sig); err != nil {
		t.Fatalf("HandleWebhook returned error: %v", err)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected 1 Upsert call, got %d", len(store.upserted))
	}
	got := store.upserted[0]
	if got.UserID != "user-abc" || got.StripeCustomerID != "cus_123" || got.StripeSubscriptionID != "sub_123" || got.Status != "active" {
		t.Errorf("Upsert record = %+v, want UserID=user-abc StripeCustomerID=cus_123 StripeSubscriptionID=sub_123 Status=active", got)
	}
}

func TestHandleWebhook_SubscriptionUpdated(t *testing.T) {
	store := &fakeStore{}
	c := newTestClient(store)

	periodEnd := time.Now().Add(30 * 24 * time.Hour).Unix()
	payload := []byte(fmt.Sprintf(`{
		"id": "evt_2",
		"type": "customer.subscription.updated",
		"data": {
			"object": {
				"id": "sub_123",
				"customer": "cus_123",
				"status": "past_due",
				"current_period_end": %d
			}
		}
	}`, periodEnd))
	sig := signPayload(t, payload, testWebhookSecret, time.Now().Unix())

	if err := c.HandleWebhook(payload, sig); err != nil {
		t.Fatalf("HandleWebhook returned error: %v", err)
	}
	if len(store.statusCalls) != 1 {
		t.Fatalf("expected 1 UpdateStatus call, got %d", len(store.statusCalls))
	}
	got := store.statusCalls[0]
	if got.stripeSubscriptionID != "sub_123" || got.status != "past_due" {
		t.Errorf("UpdateStatus call = %+v, want stripeSubscriptionID=sub_123 status=past_due", got)
	}
}

func TestHandleWebhook_SubscriptionDeleted(t *testing.T) {
	store := &fakeStore{}
	c := newTestClient(store)

	payload := []byte(`{
		"id": "evt_3",
		"type": "customer.subscription.deleted",
		"data": {
			"object": {
				"id": "sub_123",
				"customer": "cus_123",
				"status": "canceled"
			}
		}
	}`)
	sig := signPayload(t, payload, testWebhookSecret, time.Now().Unix())

	if err := c.HandleWebhook(payload, sig); err != nil {
		t.Fatalf("HandleWebhook returned error: %v", err)
	}
	if len(store.statusCalls) != 1 || store.statusCalls[0].status != "canceled" {
		t.Fatalf("expected UpdateStatus to be called with status=canceled, got %+v", store.statusCalls)
	}
}

func TestHandleWebhook_InvalidSignature(t *testing.T) {
	store := &fakeStore{}
	c := newTestClient(store)

	payload := []byte(`{"id": "evt_4", "type": "checkout.session.completed", "data": {"object": {}}}`)
	badSig := signPayload(t, payload, "wrong-secret", time.Now().Unix())

	if err := c.HandleWebhook(payload, badSig); err == nil {
		t.Fatal("expected an error for an invalid signature")
	}
	if len(store.upserted) != 0 {
		t.Error("Store should not be called when signature verification fails")
	}
}

func TestHandleWebhook_UnrelatedEventIgnored(t *testing.T) {
	store := &fakeStore{}
	c := newTestClient(store)

	payload := []byte(`{"id": "evt_5", "type": "invoice.paid", "data": {"object": {}}}`)
	sig := signPayload(t, payload, testWebhookSecret, time.Now().Unix())

	if err := c.HandleWebhook(payload, sig); err != nil {
		t.Fatalf("HandleWebhook returned error for an unrelated event: %v", err)
	}
	if len(store.upserted) != 0 || len(store.statusCalls) != 0 {
		t.Error("Store should not be called for an unrelated event type")
	}
}

func TestIsActive(t *testing.T) {
	cases := map[string]bool{
		"active":   true,
		"trialing": true,
		"canceled": false,
		"past_due": false,
		"none":     false,
	}
	for status, want := range cases {
		if got := IsActive(status); got != want {
			t.Errorf("IsActive(%q) = %v, want %v", status, got, want)
		}
	}
}
