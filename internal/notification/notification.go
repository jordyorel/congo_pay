package notification

import (
    "context"
    "log/slog"
)

const (
    // KindP2PTransfer indicates a P2P payment event.
    KindP2PTransfer = "p2p_transfer"
)

// Message describes a notification payload.
type Message struct {
    Kind        string
    Destination string
    Body        string
}

// Notifier delivers notifications to downstream systems.
type Notifier interface {
    Send(ctx context.Context, message Message) error
}

// LoggerNotifier is a stub implementation that writes notifications to the logger.
type LoggerNotifier struct {
    logger *slog.Logger
}

// NewLoggerNotifier constructs a logging notifier stub.
func NewLoggerNotifier(logger *slog.Logger) *LoggerNotifier {
    return &LoggerNotifier{logger: logger}
}

// Send writes the message to the structured logger.
func (n *LoggerNotifier) Send(_ context.Context, message Message) error {
    if n == nil || n.logger == nil {
        return nil
    }
    n.logger.Info("notification", "kind", message.Kind, "destination", message.Destination, "body", message.Body)
    return nil
}
