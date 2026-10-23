package tripo3d

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Headers Tripo sends with every webhook delivery.
const (
	// WebhookSignatureHeader carries "t=<unix seconds>,v1=<hex HMAC>".
	WebhookSignatureHeader = "Tripo-Webhook-Signature"
	// WebhookDeliveryHeader is unique per delivery; use it to drop
	// duplicates, since failed deliveries are retried.
	WebhookDeliveryHeader = "Tripo-Webhook-Delivery"
	// WebhookEventHeader repeats WebhookEvent.Type.
	WebhookEventHeader = "Tripo-Webhook-Event"
	// WebhookIDHeader identifies the endpoint configured in the console.
	WebhookIDHeader = "Tripo-Webhook-Id"
)

// Webhook event types.
const (
	WebhookTaskCompleted = "task.completed"
	WebhookTaskFailed    = "task.failed"
	WebhookBalanceLow    = "balance.low"
)

// DefaultWebhookTolerance is how far the signature timestamp may drift from
// the current time before VerifyWebhook rejects the delivery as a replay.
const DefaultWebhookTolerance = 5 * time.Minute

// ErrWebhookSignature is returned, wrapped, by VerifyWebhook for every
// delivery that fails verification. Respond with an HTTP 4xx.
var ErrWebhookSignature = errors.New("tripo3d: webhook signature verification failed")

// WebhookOptions configures VerifyWebhook.
type WebhookOptions struct {
	// Tolerance is the accepted clock drift. Zero means
	// DefaultWebhookTolerance; a negative value disables the check.
	Tolerance time.Duration
	// Now overrides the current time, for tests.
	Now func() time.Time
}

// WebhookEvent is the JSON body of a webhook delivery.
type WebhookEvent struct {
	// ID combines the task_id and event type, e.g. "<task_id>:task.completed".
	ID string `json:"id"`
	// Type is WebhookTaskCompleted, WebhookTaskFailed or WebhookBalanceLow.
	Type string `json:"type"`
	// CreatedAt is an ISO 8601 timestamp.
	CreatedAt string `json:"created_at"`
	// Data is the task snapshot for task events, and the balance and alert
	// threshold for WebhookBalanceLow.
	Data json.RawMessage `json:"data"`
}

// Task decodes Data as a task snapshot. Only meaningful for
// WebhookTaskCompleted and WebhookTaskFailed.
func (e *WebhookEvent) Task() (*Task, error) {
	var task Task
	if err := json.Unmarshal(e.Data, &task); err != nil {
		return nil, fmt.Errorf("tripo3d: webhook data is not a task: %w", err)
	}
	return &task, nil
}

// VerifyWebhook checks that payload — the raw, unmodified request body —
// was signed by Tripo with the endpoint's signing secret (whsec_…), then
// decodes it. signatureHeader is the value of WebhookSignatureHeader.
//
// Verify before acting on the event, and read the body exactly as received:
// re-encoding parsed JSON changes the bytes and breaks the signature.
func VerifyWebhook(payload []byte, signatureHeader, secret string, opts WebhookOptions) (*WebhookEvent, error) {
	if secret == "" {
		return nil, errors.New("tripo3d: VerifyWebhook: secret is required")
	}
	var timestamp string
	var signatures [][]byte
	for _, part := range strings.Split(signatureHeader, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			if sig, err := hex.DecodeString(value); err == nil {
				signatures = append(signatures, sig)
			}
		}
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || len(signatures) == 0 {
		return nil, fmt.Errorf("%w: malformed %s header", ErrWebhookSignature, WebhookSignatureHeader)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(payload)
	expected := mac.Sum(nil)
	matched := false
	for _, sig := range signatures {
		if hmac.Equal(sig, expected) {
			matched = true
		}
	}
	if !matched {
		return nil, fmt.Errorf("%w: signature mismatch", ErrWebhookSignature)
	}

	tolerance := opts.Tolerance
	if tolerance == 0 {
		tolerance = DefaultWebhookTolerance
	}
	if tolerance > 0 {
		now := time.Now
		if opts.Now != nil {
			now = opts.Now
		}
		drift := now().Sub(time.Unix(unix, 0))
		if drift < -tolerance || drift > tolerance {
			return nil, fmt.Errorf("%w: timestamp is %s away from now", ErrWebhookSignature, drift.Round(time.Second))
		}
	}

	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("tripo3d: webhook payload is not valid JSON: %w", err)
	}
	return &event, nil
}
