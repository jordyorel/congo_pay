package auth

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "strings"
)

var (
    b64 = base64.RawURLEncoding
)

// SignHS256 creates a compact JWT string using HS256.
func SignHS256(claims map[string]any, secret []byte) (string, error) {
    header := map[string]string{"alg": "HS256", "typ": "JWT"}
    h, err := json.Marshal(header)
    if err != nil { return "", err }
    c, err := json.Marshal(claims)
    if err != nil { return "", err }
    unsigned := b64.EncodeToString(h) + "." + b64.EncodeToString(c)
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(unsigned))
    sig := mac.Sum(nil)
    return unsigned + "." + b64.EncodeToString(sig), nil
}

// ParseAndVerifyHS256 verifies token signature and returns claims.
func ParseAndVerifyHS256(token string, secret []byte) (map[string]any, error) {
    parts := strings.Split(token, ".")
    if len(parts) != 3 { return nil, errors.New("invalid token format") }
    unsigned := parts[0] + "." + parts[1]
    sigBytes, err := b64.DecodeString(parts[2])
    if err != nil { return nil, errors.New("invalid signature encoding") }
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(unsigned))
    if !hmac.Equal(sigBytes, mac.Sum(nil)) { return nil, errors.New("signature mismatch") }
    payload, err := b64.DecodeString(parts[1])
    if err != nil { return nil, errors.New("invalid payload encoding") }
    var claims map[string]any
    if err := json.Unmarshal(payload, &claims); err != nil { return nil, errors.New("invalid claims json") }
    return claims, nil
}

