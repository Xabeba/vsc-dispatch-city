package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/teko/food-delivery/internal/events"
	"github.com/teko/food-delivery/internal/model"
)

const (
	Exchange   = "food.events"
	DeadLetter = "food.dlx"
	DeadQueue  = "food.dead"
)

// Publisher publishes confirmed persistent messages to the topic exchange.
type Publisher struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

// NewPublisher connects to RabbitMQ and declares the shared topology.
func NewPublisher(ctx context.Context, url string, logger *slog.Logger) (*Publisher, error) {
	connection, err := dial(ctx, url, logger)
	if err != nil {
		return nil, err
	}
	channel, err := connection.Channel()
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("open publisher channel: %w", err)
	}
	if err := declareExchanges(channel); err != nil {
		channel.Close()
		connection.Close()
		return nil, err
	}
	if err := channel.Confirm(false); err != nil {
		channel.Close()
		connection.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}
	return &Publisher{connection: connection, channel: channel}, nil
}

// Publish sends one event and waits for the broker confirmation.
func (p *Publisher) Publish(ctx context.Context, event model.EventEnvelope) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(ctx, Exchange, events.RoutingKey(event), false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    event.ID,
		Timestamp:    event.OccurredAt,
		Body:         raw,
	})
	if err != nil {
		return fmt.Errorf("publish %s: %w", event.Type, err)
	}
	confirmed, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for publish confirmation: %w", err)
	}
	if !confirmed {
		return fmt.Errorf("broker rejected event %s", event.ID)
	}
	return nil
}

// Close releases the AMQP resources.
func (p *Publisher) Close() error {
	if err := p.channel.Close(); err != nil {
		return fmt.Errorf("close publisher channel: %w", err)
	}
	if err := p.connection.Close(); err != nil {
		return fmt.Errorf("close publisher connection: %w", err)
	}
	return nil
}

// ConsumerConfig describes a durable or ephemeral subscription.
type ConsumerConfig struct {
	Queue      string
	Bindings   []string
	Workers    int
	Prefetch   int
	Exclusive  bool
	AutoDelete bool
}

// Handler processes one event. Returning an error dead-letters the message.
type Handler func(context.Context, model.EventEnvelope) error

// Consume connects, declares the queue and blocks until context cancellation.
func Consume(ctx context.Context, url string, config ConsumerConfig, logger *slog.Logger, handler Handler) error {
	connection, err := dial(ctx, url, logger)
	if err != nil {
		return err
	}
	defer connection.Close()
	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("open consumer channel: %w", err)
	}
	defer channel.Close()
	if err := declareExchanges(channel); err != nil {
		return err
	}
	arguments := amqp.Table{"x-dead-letter-exchange": DeadLetter}
	queue, err := channel.QueueDeclare(config.Queue, !config.Exclusive, config.AutoDelete, config.Exclusive, false, arguments)
	if err != nil {
		return fmt.Errorf("declare queue %s: %w", config.Queue, err)
	}
	for _, binding := range config.Bindings {
		if err := channel.QueueBind(queue.Name, binding, Exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue %s to %s: %w", queue.Name, binding, err)
		}
	}
	if config.Prefetch <= 0 {
		config.Prefetch = 1
	}
	if err := channel.Qos(config.Prefetch, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}
	deliveries, err := channel.Consume(queue.Name, "", false, config.Exclusive, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume queue %s: %w", queue.Name, err)
	}
	if config.Workers <= 0 {
		config.Workers = 1
	}

	errCh := make(chan error, config.Workers)
	for worker := 0; worker < config.Workers; worker++ {
		go func(workerID int) {
			for {
				select {
				case <-ctx.Done():
					return
				case delivery, ok := <-deliveries:
					if !ok {
						errCh <- fmt.Errorf("delivery channel closed")
						return
					}
					var event model.EventEnvelope
					if err := json.Unmarshal(delivery.Body, &event); err != nil {
						logger.Error("invalid event moved to DLQ", "worker", workerID, "error", err)
						if nackErr := delivery.Nack(false, false); nackErr != nil {
							errCh <- fmt.Errorf("nack invalid message: %w", nackErr)
							return
						}
						continue
					}
					if err := handler(ctx, event); err != nil {
						if errors.Is(err, context.Canceled) {
							if nackErr := delivery.Nack(false, true); nackErr != nil {
								errCh <- fmt.Errorf("requeue interrupted event: %w", nackErr)
								return
							}
							continue
						}
						logger.Error("event handler failed", "event_id", event.ID, "event_type", event.Type, "error", err)
						if nackErr := delivery.Nack(false, false); nackErr != nil {
							errCh <- fmt.Errorf("nack event: %w", nackErr)
							return
						}
						continue
					}
					if err := delivery.Ack(false); err != nil {
						errCh <- fmt.Errorf("ack event: %w", err)
						return
					}
				}
			}
		}(worker)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("consumer stopped: %w", ctx.Err())
	case err := <-errCh:
		return err
	}
}

func declareExchanges(channel *amqp.Channel) error {
	if err := channel.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare event exchange: %w", err)
	}
	if err := channel.ExchangeDeclare(DeadLetter, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dead letter exchange: %w", err)
	}
	if _, err := channel.QueueDeclare(DeadQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dead letter queue: %w", err)
	}
	if err := channel.QueueBind(DeadQueue, "#", DeadLetter, false, nil); err != nil {
		return fmt.Errorf("bind dead letter queue: %w", err)
	}
	return nil
}

func dial(ctx context.Context, url string, logger *slog.Logger) (*amqp.Connection, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for attempt := 1; ; attempt++ {
		connection, err := amqp.DialConfig(url, amqp.Config{Heartbeat: 10 * time.Second, Dial: amqp.DefaultDial(5 * time.Second)})
		if err == nil {
			return connection, nil
		}
		logger.Warn("waiting for RabbitMQ", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("connect to RabbitMQ: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
