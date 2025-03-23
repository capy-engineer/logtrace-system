package nats

import (
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// natsClient encapsulates NATS connection and JetStream context
type natsClient struct {
	Conn      *nats.Conn
	JS        nats.JetStreamContext
	StreamCfg *nats.StreamConfig
}

// Config holds configuration for NATS client
type Config struct {
	URL             string
	ReconnectWait   time.Duration
	MaxReconnects   int
	ConnectionName  string
	StreamName      string
	StreamSubjects  []string
	RetentionPolicy nats.RetentionPolicy
	StorageType     nats.StorageType
	MaxAge          time.Duration
	Replicas        int
}

// NewClient creates a new NATS client with JetStream enabled
func NewClient(config Config) (*natsClient, error) {
	// Define connection options
	opts := []nats.Option{
		nats.Name(config.ConnectionName),
		nats.ReconnectWait(config.ReconnectWait),
		nats.MaxReconnects(config.MaxReconnects),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("NATS disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("NATS reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("NATS error: %v", err)
		}),
	}

	// Connect to NATS
	nc, err := nats.Connect(config.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	client := &natsClient{
		Conn: nc,
		JS:   js,
	}

	// Set up logs stream if configured
	if config.StreamName != "" {
		err = client.SetupStream(config)
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("failed to set up stream: %w", err)
		}
	}

	return client, nil
}

// SetupStream creates or updates a JetStream stream
func (c *natsClient) SetupStream(config Config) error {
	// Check if stream exists
	_, err := c.JS.StreamInfo(config.StreamName)
	if err != nil {
		// Stream doesn't exist, create it
		streamConfig := &nats.StreamConfig{
			Name:      config.StreamName,
			Subjects:  config.StreamSubjects,
			Retention: config.RetentionPolicy,
			MaxAge:    config.MaxAge,
			Storage:   config.StorageType,
			Replicas:  config.Replicas,
			NoAck:     false,
			Discard:   nats.DiscardOld,
			MaxMsgs:   -1,
			MaxBytes:  -1,
		}

		_, err = c.JS.AddStream(streamConfig)
		if err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}
		log.Printf("Stream %s created", config.StreamName)
		c.StreamCfg = streamConfig
	} else {
		// Stream exists, update it
		streamConfig := &nats.StreamConfig{
			Name:      config.StreamName,
			Subjects:  config.StreamSubjects,
			Retention: config.RetentionPolicy,
			MaxAge:    config.MaxAge,
			Storage:   config.StorageType,
			Replicas:  config.Replicas,
			NoAck:     false,
			Discard:   nats.DiscardOld,
			MaxMsgs:   -1,
			MaxBytes:  -1,
		}

		_, err = c.JS.UpdateStream(streamConfig)
		if err != nil {
			return fmt.Errorf("failed to update stream: %w", err)
		}
		log.Printf("Stream %s updated", config.StreamName)
		c.StreamCfg = streamConfig
	}

	return nil
}

// Close gracefully shuts down the NATS connection
func (c *natsClient) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

// Publish publishes a message to the specified subject
func (c *natsClient) Publish(subject string, data []byte) (*nats.PubAck, error) {
	return c.JS.Publish(subject, data)
}

// CreatePullConsumer creates a pull consumer if it doesn't already exist
func (c *natsClient) CreatePullConsumer(name string, filterSubject string) error {
	if c.StreamCfg == nil {
		return fmt.Errorf("stream not set up; call SetupStream first")
	}

	// Check if consumer exists
	_, err := c.JS.ConsumerInfo(c.StreamCfg.Name, name)
	if err != nil {
		// Consumer doesn't exist, create it
		_, err = c.JS.AddConsumer(c.StreamCfg.Name, &nats.ConsumerConfig{
			Durable:       name,
			AckPolicy:     nats.AckExplicitPolicy,
			FilterSubject: filterSubject,
			MaxDeliver:    -1,
		})
		if err != nil {
			return fmt.Errorf("failed to create consumer: %w", err)
		}
		log.Printf("Consumer %s created", name)
	}

	return nil
}

// CreatePushConsumer creates a push consumer with a message handler
func (c *natsClient) CreatePushConsumer(name string, filterSubject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	if c.StreamCfg == nil {
		return nil, fmt.Errorf("stream not set up; call SetupStream first")
	}

	// Create a push subscription
	sub, err := c.JS.Subscribe(
		filterSubject,
		handler,
		nats.Durable(name),
		nats.AckExplicit(),
		nats.DeliverAll(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create push subscription: %w", err)
	}

	log.Printf("Push consumer %s created", name)
	return sub, nil
}

// SubscribePull subscribes to a pull consumer and returns the subscription
func (c *natsClient) SubscribePull(consumerName string, filterSubject string) (*nats.Subscription, error) {
	if c.StreamCfg == nil {
		return nil, fmt.Errorf("stream not set up; call SetupStream first")
	}

	// Make sure the consumer exists
	err := c.CreatePullConsumer(consumerName, filterSubject)
	if err != nil {
		return nil, err
	}

	// Create pull subscription
	sub, err := c.JS.PullSubscribe(
		filterSubject,
		consumerName,
		nats.Bind(c.StreamCfg.Name, consumerName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull subscription: %w", err)
	}

	log.Printf("Pull subscription for consumer %s created", consumerName)
	return sub, nil
}

// RequestReply demonstrates standard NATS request-reply pattern (non-JetStream)
func (c *natsClient) RequestReply(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	return c.Conn.Request(subject, data, timeout)
}

// ListStreams lists all streams in the JetStream server
func (c *natsClient) ListStreams() ([]*nats.StreamInfo, error) {
	var results []*nats.StreamInfo

	// Get a stream context
	ctx := c.JS.StreamsInfo()

	// Iterate through all streams
	for info := range ctx {
		if info == nil {
			return nil, fmt.Errorf("error receiving stream info")
		}
		results = append(results, info)
	}

	return results, nil
}
