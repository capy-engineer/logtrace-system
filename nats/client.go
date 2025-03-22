package nats

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"log"
	"time"
)

type natsClient struct {
	conn      *nats.Conn
	Js        nats.JetStreamContext
	streamCfg *nats.StreamConfig
}

func NewNATSClient(url string, options ...nats.Option) (*natsClient, error) {
	// Connect to NATS
	nc, err := nats.Connect(url, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &natsClient{
		conn: nc,
		Js:   js,
	}, nil
}
func (c *natsClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *natsClient) SetupStream(config *nats.StreamConfig) error {
	// Check if stream exists
	_, err := c.Js.StreamInfo(config.Name)
	if err != nil {
		// Stream doesn't exist, create it
		_, err = c.Js.AddStream(config)
		if err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}
		log.Printf("Stream %s created", config.Name)
	} else {
		// Stream exists, update it
		_, err = c.Js.UpdateStream(config)
		if err != nil {
			return fmt.Errorf("failed to update stream: %w", err)
		}
		log.Printf("Stream %s updated", config.Name)
	}

	c.streamCfg = config
	return nil
}

func (c *natsClient) Publish(subject string, data []byte) (*nats.PubAck, error) {
	return c.Js.Publish(subject, data)
}

func (c *natsClient) CreatePullConsumer(name string, filterSubject string) error {
	if c.streamCfg == nil {
		return fmt.Errorf("stream not set up; call SetupStream first")
	}

	_, err := c.Js.ConsumerInfo(c.streamCfg.Name, name)
	if err != nil {
		// Consumer doesn't exist, create it
		_, err = c.Js.AddConsumer(c.streamCfg.Name, &nats.ConsumerConfig{
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

func (c *natsClient) CreatePushConsumer(name string, filterSubject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	if c.streamCfg == nil {
		return nil, fmt.Errorf("stream not set up; call SetupStream first")
	}

	// Create a push subscription
	sub, err := c.Js.Subscribe(
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

func (c *natsClient) SubscribePull(consumerName string, filterSubject string) (*nats.Subscription, error) {
	if c.streamCfg == nil {
		return nil, fmt.Errorf("stream not set up; call SetupStream first")
	}

	err := c.CreatePullConsumer(consumerName, filterSubject)
	if err != nil {
		return nil, err
	}

	// Create pull subscription
	sub, err := c.Js.PullSubscribe(
		filterSubject,
		consumerName,
		nats.Bind(c.streamCfg.Name, consumerName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull subscription: %w", err)
	}

	log.Printf("Pull subscription for consumer %s created", consumerName)
	return sub, nil
}

func (c *natsClient) RequestReply(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	return c.conn.Request(subject, data, timeout)
}

func (c *natsClient) ListStreams() ([]*nats.StreamInfo, error) {
	var results []*nats.StreamInfo

	// Get a stream context
	ctx := c.Js.StreamsInfo()

	// Iterate through all streams
	for info := range ctx {
		if info == nil {
			return nil, fmt.Errorf("error receiving stream info")
		}
		results = append(results, info)
	}

	return results, nil
}
