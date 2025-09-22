package funding

import (
	"context"

	"github.com/google/uuid"
)

// Acquirer represents a connector to an external card processor.
type Acquirer interface {
	AuthorizeCardIn(ctx context.Context, input CardInAuthorization) (AuthorizationDecision, error)
	AuthorizeCardOut(ctx context.Context, input CardOutAuthorization) (AuthorizationDecision, error)
}

// AuthorizationDecision captures the simulated response from the acquirer.
type AuthorizationDecision struct {
	Reference string
	Status    string
}

// CardInAuthorization encapsulates details needed for a card top-up authorization.
type CardInAuthorization struct {
	CardNumber string
	Expiry     string
	CVV        string
	Amount     int64
}

// CardOutAuthorization captures data for a push-to-card payout authorization.
type CardOutAuthorization struct {
	CardNumber string
	Amount     int64
}

// StaticAcquirer simulates a successful acquirer integration.
type StaticAcquirer struct{}

// AuthorizeCardIn approves the funding request with a synthetic reference.
func (StaticAcquirer) AuthorizeCardIn(_ context.Context, _ CardInAuthorization) (AuthorizationDecision, error) {
	return AuthorizationDecision{Reference: uuid.NewString(), Status: "approved"}, nil
}

// AuthorizeCardOut approves the withdrawal request with a synthetic reference.
func (StaticAcquirer) AuthorizeCardOut(_ context.Context, _ CardOutAuthorization) (AuthorizationDecision, error) {
	return AuthorizationDecision{Reference: uuid.NewString(), Status: "approved"}, nil
}
