package identity

import "time"

// User represents a registered wallet owner.
type User struct {
    ID        string
    Phone     string
    Tier      string
    PINHash   []byte
    DeviceID  string
    CreatedAt time.Time
}

// Credentials request structure.
type Credentials struct {
    Phone    string
    PIN      string
    DeviceID string
}
