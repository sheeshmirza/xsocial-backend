package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"xsocial-backend/internal/utils"
)

const (
	ExchangeDirect = "xsocial.direct"
	ExchangeEvents = "xsocial.events"
	ExchangeDLX    = "xsocial.dlx"

	QueuePostPublish = "xsocial.posts.publish"
	QueuePostDLQ     = "xsocial.posts.dlq"

	RoutingPostPublish = "post.publish"
	RoutingPostDead    = "post.publish.dead"
)

// RabbitMQBroker manages high-reliability AMQP message routing, channels, and consumer topologies.
type RabbitMQBroker struct {
	mu         sync.RWMutex
	url        string
	conn       *amqp.Connection
	pubChannel *amqp.Channel
	closed     bool
	log        *slog.Logger
}

// NewRabbitMQBroker connects to RabbitMQ and establishes the core durable exchange/queue topology.
func NewRabbitMQBroker(url string) (*RabbitMQBroker, error) {
	if url == "" {
		return nil, fmt.Errorf("rabbitmq URL is empty")
	}

	logger := slog.Default().With("component", "rabbitmq")
	broker := &RabbitMQBroker{
		url: url,
		log: logger,
	}

	if err := broker.connect(); err != nil {
		return nil, err
	}

	if err := broker.setupTopology(); err != nil {
		_ = broker.Close()
		return nil, fmt.Errorf("failed to configure rabbitmq topology: %w", err)
	}

	logger.Info("rabbitmq_connected_and_topology_initialized", "url", utils.MaskURL(url))
	return broker, nil
}

func (b *RabbitMQBroker) connect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	conn, err := amqp.Dial(b.url)
	if err != nil {
		return fmt.Errorf("failed to dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to open rabbitmq channel: %w", err)
	}

	b.conn = conn
	b.pubChannel = ch
	b.closed = false
	return nil
}

func (b *RabbitMQBroker) setupTopology() error {
	b.mu.RLock()
	ch := b.pubChannel
	b.mu.RUnlock()

	if ch == nil {
		return fmt.Errorf("channel is uninitialized")
	}

	// 1. Declare Dead Letter Exchange
	err := ch.ExchangeDeclare(
		ExchangeDLX,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("exchange declare %s: %w", ExchangeDLX, err)
	}

	// 2. Declare Dead Letter Queue
	_, err = ch.QueueDeclare(
		QueuePostDLQ,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("queue declare %s: %w", QueuePostDLQ, err)
	}

	// Bind DLQ to DLX
	if err := ch.QueueBind(QueuePostDLQ, RoutingPostDead, ExchangeDLX, false, nil); err != nil {
		return fmt.Errorf("queue bind %s to %s: %w", QueuePostDLQ, ExchangeDLX, err)
	}

	// 3. Declare Primary Direct Exchange
	err = ch.ExchangeDeclare(
		ExchangeDirect,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("exchange declare %s: %w", ExchangeDirect, err)
	}

	// 4. Declare Primary Post Publish Queue with Dead Letter routing
	args := amqp.Table{
		"x-dead-letter-exchange":    ExchangeDLX,
		"x-dead-letter-routing-key": RoutingPostDead,
	}
	_, err = ch.QueueDeclare(
		QueuePostPublish,
		true,
		false,
		false,
		false,
		args,
	)
	if err != nil {
		return fmt.Errorf("queue declare %s: %w", QueuePostPublish, err)
	}

	// Bind Primary Queue to Direct Exchange
	if err := ch.QueueBind(QueuePostPublish, RoutingPostPublish, ExchangeDirect, false, nil); err != nil {
		return fmt.Errorf("queue bind %s to %s: %w", QueuePostPublish, ExchangeDirect, err)
	}

	return nil
}

// Publish sends a structured message to an exchange with persistent delivery mode and tracing metadata.
func (b *RabbitMQBroker) Publish(ctx context.Context, exchange, routingKey string, payload any) error {
	if b == nil {
		return fmt.Errorf("rabbitmq broker is uninitialized")
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed || b.pubChannel == nil {
		return fmt.Errorf("rabbitmq publisher is closed or unavailable")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal rabbitmq payload: %w", err)
	}

	traceID := utils.TraceIDFromContext(ctx)
	reqID := utils.RequestIDFromContext(ctx)

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Timestamp:    time.Now().UTC(),
		MessageId:    reqID,
		CorrelationId: traceID,
		Body:         body,
		Headers: amqp.Table{
			"traceparent":  utils.TraceparentFromContext(ctx),
			"request_id":   reqID,
			"published_at": time.Now().UTC().Format(time.RFC3339),
		},
	}

	return b.pubChannel.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		msg,
	)
}

// Consume begins consuming from the specified queue with prefetch QoS and concurrent worker distribution.
func (b *RabbitMQBroker) Consume(ctx context.Context, queueName string, prefetchCount int, handler func(ctx context.Context, delivery amqp.Delivery) error) error {
	if b == nil {
		return fmt.Errorf("rabbitmq broker is uninitialized")
	}
	b.mu.RLock()
	conn := b.conn
	b.mu.RUnlock()

	if conn == nil || conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection is closed")
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open consumer channel: %w", err)
	}

	if prefetchCount <= 0 {
		prefetchCount = 10
	}
	if err := ch.Qos(prefetchCount, 0, false); err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to set consumer qos: %w", err)
	}

	deliveries, err := ch.Consume(
		queueName,
		"",    // consumer tag
		false, // auto-ack (explicit Ack required for reliable message delivery)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to start consumer on queue %s: %w", queueName, err)
	}

	utils.SafeGo(func() {
		defer ch.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-deliveries:
				if !ok {
					return
				}

				// Reconstruct trace context from headers
				msgCtx := ctx
				tpVal, _ := msg.Headers["traceparent"].(string)
				ridVal, _ := msg.Headers["request_id"].(string)
				if tpVal != "" || ridVal != "" {
					msgCtx = utils.ContextWithTraceparent(msgCtx, tpVal, ridVal)
				}

				if err := handler(msgCtx, msg); err != nil {
					b.log.Error("rabbitmq_consumer_message_failed", "queue", queueName, "error", err)
					// Reject and route to dead letter exchange
					_ = msg.Nack(false, false)
				} else {
					_ = msg.Ack(false)
				}
			}
		}
	})

	return nil
}

// Health checks if the RabbitMQ connection is active.
func (b *RabbitMQBroker) Health(ctx context.Context) error {
	if b == nil {
		return fmt.Errorf("rabbitmq broker is uninitialized")
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed || b.conn == nil || b.conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection is down or uninitialized")
	}
	return nil
}

// Close gracefully closes publisher channels and the connection.
func (b *RabbitMQBroker) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}
	b.closed = true

	var err error
	if b.pubChannel != nil {
		_ = b.pubChannel.Close()
	}
	if b.conn != nil && !b.conn.IsClosed() {
		err = b.conn.Close()
	}
	return err
}
