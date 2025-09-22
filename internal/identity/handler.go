package identity

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// Handler exposes identity endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs an identity HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type registerRequest struct {
	Phone    string `json:"phone"`
	PIN      string `json:"pin"`
	DeviceID string `json:"device_id"`
}

type authResponse struct {
	UserID   string `json:"user_id"`
	Phone    string `json:"phone"`
	Tier     string `json:"tier"`
	DeviceID string `json:"device_id"`
}

// Register handles user onboarding.
func (h *Handler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	user, err := h.service.Register(c.UserContext(), Credentials{Phone: req.Phone, PIN: req.PIN, DeviceID: req.DeviceID})
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	return c.Status(http.StatusCreated).JSON(authResponse{UserID: user.ID, Phone: user.Phone, Tier: user.Tier, DeviceID: user.DeviceID})
}

// Authenticate verifies login credentials.
func (h *Handler) Authenticate(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	user, err := h.service.Authenticate(c.UserContext(), Credentials{Phone: req.Phone, PIN: req.PIN, DeviceID: req.DeviceID})
	if err != nil {
		return fiber.NewError(http.StatusUnauthorized, err.Error())
	}
	return c.Status(http.StatusOK).JSON(authResponse{UserID: user.ID, Phone: user.Phone, Tier: user.Tier, DeviceID: user.DeviceID})
}
