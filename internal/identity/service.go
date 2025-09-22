package identity

import (
    "context"
    "errors"
    "time"

    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
)

const (
    tierZero = "tier0"
    tierOne  = "tier1"
)

// Service manages identity lifecycle.
type Service struct {
    repo Repository
}

// NewService creates a new identity service.
func NewService(repo Repository) *Service {
    return &Service{repo: repo}
}

// Register creates a new Tier0 user and stores a hashed PIN.
func (s *Service) Register(ctx context.Context, creds Credentials) (User, error) {
    if len(creds.PIN) < 4 {
        return User{}, errors.New("PIN must be at least 4 digits")
    }

    hash, err := bcrypt.GenerateFromPassword([]byte(creds.PIN), bcrypt.DefaultCost)
    if err != nil {
        return User{}, err
    }

    user := User{
        ID:        uuid.New().String(),
        Phone:     creds.Phone,
        Tier:      tierZero,
        PINHash:   hash,
        DeviceID:  creds.DeviceID,
        CreatedAt: time.Now().UTC(),
    }

    if err := s.repo.Create(ctx, user); err != nil {
        return User{}, err
    }

    return user, nil
}

// Authenticate verifies credentials and device binding.
func (s *Service) Authenticate(ctx context.Context, creds Credentials) (User, error) {
    user, err := s.repo.FindByPhone(ctx, creds.Phone)
    if err != nil {
        return User{}, err
    }

    if err := bcrypt.CompareHashAndPassword(user.PINHash, []byte(creds.PIN)); err != nil {
        return User{}, errors.New("invalid PIN")
    }

    if user.DeviceID == "" {
        if creds.DeviceID == "" {
            return User{}, errors.New("device binding required")
        }
        if err := s.repo.UpdateDevice(ctx, user.ID, creds.DeviceID); err != nil {
            return User{}, err
        }
        user.DeviceID = creds.DeviceID
    } else if creds.DeviceID != "" && user.DeviceID != creds.DeviceID {
        return User{}, errors.New("device mismatch")
    }

    if user.Tier == tierZero {
        user.Tier = tierOne
    }

    return user, nil
}
