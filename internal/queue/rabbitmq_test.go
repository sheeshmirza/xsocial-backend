package queue

import (
	"context"
	"testing"
)

func TestRabbitMQBroker_EmptyURL(t *testing.T) {
	broker, err := NewRabbitMQBroker("")
	if err == nil {
		t.Errorf("expected error for empty URL, got nil")
	}
	if broker != nil {
		t.Errorf("expected nil broker on error, got %v", broker)
	}
}

func TestRabbitMQBroker_NilSafety(t *testing.T) {
	var broker *RabbitMQBroker
	ctx := context.Background()

	if err := broker.Health(ctx); err == nil {
		t.Errorf("expected error for nil broker Health(), got nil")
	}

	if err := broker.Close(); err != nil {
		t.Errorf("expected nil error closing nil broker, got %v", err)
	}

	if err := broker.Publish(ctx, "test", "key", nil); err == nil {
		t.Errorf("expected error publishing on nil broker, got nil")
	}
}
