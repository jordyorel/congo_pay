package auth

import (
    "context"
    "errors"
    "time"

    "github.com/congo-pay/congo_pay/internal/config"
    "github.com/congo-pay/congo_pay/internal/identity"
)

type Service struct {
    cfg    config.Config
    idRepo identity.Repository
}

func NewService(cfg config.Config, idRepo identity.Repository) *Service {
    return &Service{cfg: cfg, idRepo: idRepo}
}

type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}

// Login validates credentials (by delegating to identity.Service) and issues tokens.
func (s *Service) Login(user identity.User) (TokenPair, error) {
    access, accessExp, err := s.sign(user, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
    if err != nil {
        return TokenPair{}, err
    }
    refresh, _, err := s.sign(user, s.cfg.RefreshSecret, s.cfg.RefreshTokenTTL)
    if err != nil {
        return TokenPair{}, err
    }
    return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(accessExp.Sub(time.Now()).Seconds())}, nil
}

func (s *Service) sign(user identity.User, secret string, ttl time.Duration) (string, time.Time, error) {
    now := time.Now()
    exp := now.Add(ttl)
    claims := map[string]any{
        "sub": user.ID,
        "phone": user.Phone,
        "tier": user.Tier,
        "ver": user.TokenVersion,
        "iat": now.Unix(),
        "exp": exp.Unix(),
    }
    signed, err := SignHS256(claims, []byte(secret))
    if err != nil {
        return "", time.Time{}, err
    }
    return signed, exp, nil
}

// Refresh verifies the refresh token and returns a new access token if valid.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (string, int64, error) {
    claims, err := ParseAndVerifyHS256(refreshToken, []byte(s.cfg.RefreshSecret))
    if err != nil {
        return "", 0, errors.New("invalid refresh token")
    }
    sub, _ := claims["sub"].(string)
    verFloat, _ := claims["ver"].(float64)
    ver := int(verFloat)

    user, err := s.idRepo.FindByID(ctx, sub)
    if err != nil {
        return "", 0, errors.New("user not found")
    }
    if user.TokenVersion != ver {
        return "", 0, errors.New("token version invalidated")
    }

    // Issue new access token with same version
    accessClaims := map[string]any{
        "sub": sub,
        "ver": ver,
        "iat": time.Now().Unix(),
        "exp": time.Now().Add(s.cfg.AccessTokenTTL).Unix(),
    }
    signed, err := SignHS256(accessClaims, []byte(s.cfg.JWTSecret))
    if err != nil {
        return "", 0, err
    }
    return signed, int64(s.cfg.AccessTokenTTL.Seconds()), nil
}

// Logout increments token version so older tokens become invalid.
func (s *Service) Logout(ctx context.Context, userID string) error {
    user, err := s.idRepo.FindByID(ctx, userID)
    if err != nil {
        return err
    }
    return s.idRepo.UpdateTokenVersion(ctx, user.ID, user.TokenVersion+1)
}
