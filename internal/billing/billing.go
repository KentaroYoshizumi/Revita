// Package billing handles Revita's ¥300/month Stripe subscription:
// starting a Checkout session for a signed-in user, and applying
// webhook events to keep subscription status in sync.
//
// Webhook event payloads are decoded into small local structs rather
// than the full stripe-go SDK types, since Stripe's subscription object
// shape (e.g. where current_period_end lives) has changed across API
// versions; this keeps webhook parsing stable and easy to verify
// against whatever payload Stripe actually sends. Session creation
// still goes through the official SDK, since getting the request shape
// right matters more there than avoiding version drift.
package billing

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
)

// SubscriptionRecord is the subscription state billing persists.
type SubscriptionRecord struct {
	UserID               string
	StripeCustomerID     string
	StripeSubscriptionID string
	Status               string // Stripe subscription status, e.g. "active", "canceled"
	CurrentPeriodEnd     time.Time
}

// SubscriptionStore persists subscription state. The Postgres-backed
// implementation lives in internal/db; tests use an in-memory fake.
type SubscriptionStore interface {
	// Upsert links a Stripe customer/subscription to a Revita user.
	// Only checkout.session.completed carries our own user ID (via
	// client_reference_id), so this is the one place a new row is
	// created.
	Upsert(rec SubscriptionRecord) error
	// UpdateStatus updates an existing subscription's status and
	// period end, looked up by Stripe subscription ID — used for
	// customer.subscription.updated/deleted, which only carry Stripe's
	// own IDs.
	UpdateStatus(stripeSubscriptionID, status string, currentPeriodEnd time.Time) error
}

// IsActive reports whether a subscription status permits running
// evaluations.
func IsActive(status string) bool {
	return status == "active" || status == "trialing"
}

// Client is Revita's Stripe integration.
type Client struct {
	PriceID       string // Stripe Price ID for the ¥300/month plan
	WebhookSecret string
	SuccessURL    string
	CancelURL     string
	Store         SubscriptionStore
}

// NewClient configures the Stripe SDK with apiKey and returns a Client
// for the given monthly Price ID.
func NewClient(apiKey, priceID, webhookSecret, successURL, cancelURL string, store SubscriptionStore) *Client {
	stripe.Key = apiKey
	return &Client{
		PriceID:       priceID,
		WebhookSecret: webhookSecret,
		SuccessURL:    successURL,
		CancelURL:     cancelURL,
		Store:         store,
	}
}

// CreateCheckoutSession starts a Stripe Checkout flow for userID's
// ¥300/month subscription. ClientReferenceID carries the Revita user ID
// through to the checkout.session.completed webhook, since Stripe has
// no notion of our own user accounts.
func (c *Client) CreateCheckoutSession(userID, userEmail string) (string, error) {
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(c.PriceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:        stripe.String(c.SuccessURL),
		CancelURL:         stripe.String(c.CancelURL),
		ClientReferenceID: stripe.String(userID),
		CustomerEmail:     stripe.String(userEmail),
	}

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("billing: failed to create checkout session: %w", err)
	}
	return sess.URL, nil
}

// checkoutSessionCompleted is the subset of Stripe's
// checkout.session.completed payload billing needs.
type checkoutSessionCompleted struct {
	Customer          string `json:"customer"`
	Subscription      string `json:"subscription"`
	ClientReferenceID string `json:"client_reference_id"`
}

// subscriptionEvent is the subset of Stripe's customer.subscription.*
// payload billing needs.
//
// NOTE: current_period_end has moved around across Stripe API versions
// (subscription-level vs. per-subscription-item). Verify this field is
// still present at this path for your Stripe API version before relying
// on it; if not, read it from items.data[0].current_period_end instead.
type subscriptionEvent struct {
	ID               string `json:"id"`
	Customer         string `json:"customer"`
	Status           string `json:"status"`
	CurrentPeriodEnd int64  `json:"current_period_end"`
}

// HandleWebhook verifies payload's Stripe-Signature header and applies
// the event to the subscription store.
//
// It ignores the stripe-go SDK's API-version compatibility check:
// webhook payloads are decoded into this package's own minimal structs
// rather than the SDK's typed objects, so a Stripe account pinned to a
// different API version than this SDK build still parses correctly —
// only the signature (a version-independent HMAC over the raw payload)
// needs to match.
func (c *Client) HandleWebhook(payload []byte, sigHeader string) error {
	event, err := webhook.ConstructEventWithOptions(payload, sigHeader, c.WebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return fmt.Errorf("billing: signature verification failed: %w", err)
	}

	switch event.Type {
	case "checkout.session.completed":
		var sess checkoutSessionCompleted
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return fmt.Errorf("billing: failed to parse checkout.session.completed: %w", err)
		}
		if sess.ClientReferenceID == "" {
			return fmt.Errorf("billing: checkout session missing client_reference_id")
		}
		return c.Store.Upsert(SubscriptionRecord{
			UserID:               sess.ClientReferenceID,
			StripeCustomerID:     sess.Customer,
			StripeSubscriptionID: sess.Subscription,
			Status:               "active",
		})

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub subscriptionEvent
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return fmt.Errorf("billing: failed to parse %s: %w", event.Type, err)
		}
		status := sub.Status
		if event.Type == "customer.subscription.deleted" {
			status = "canceled"
		}
		var periodEnd time.Time
		if sub.CurrentPeriodEnd > 0 {
			periodEnd = time.Unix(sub.CurrentPeriodEnd, 0)
		}
		return c.Store.UpdateStatus(sub.ID, status, periodEnd)

	default:
		// Other event types (e.g. invoice.*) aren't needed to keep
		// subscription status in sync; ignore them.
		return nil
	}
}
