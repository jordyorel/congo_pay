package identity

import (
    "context"
    "testing"
)

func TestRegisterAndAuthenticate(t *testing.T) {
    repo := NewMemoryRepository()
    svc := NewService(repo)

    ctx := context.Background()
    user, err := svc.Register(ctx, Credentials{Phone: "+237650000000", PIN: "1234", DeviceID: "device-1"})
    if err != nil {
        t.Fatalf("register: %v", err)
    }

    if user.Tier != tierZero {
        t.Fatalf("expected tier0, got %s", user.Tier)
    }

    authed, err := svc.Authenticate(ctx, Credentials{Phone: user.Phone, PIN: "1234", DeviceID: "device-1"})
    if err != nil {
        t.Fatalf("authenticate: %v", err)
    }
    if authed.Tier != tierOne {
        t.Fatalf("expected promotion to tier1, got %s", authed.Tier)
    }
}

func TestAuthenticateDeviceMismatch(t *testing.T) {
    repo := NewMemoryRepository()
    svc := NewService(repo)
    ctx := context.Background()

    _, err := svc.Register(ctx, Credentials{Phone: "123", PIN: "1234", DeviceID: "device-1"})
    if err != nil {
        t.Fatalf("register: %v", err)
    }

    if _, err := svc.Authenticate(ctx, Credentials{Phone: "123", PIN: "1234", DeviceID: "device-2"}); err == nil {
        t.Fatalf("expected device mismatch error")
    }
}
