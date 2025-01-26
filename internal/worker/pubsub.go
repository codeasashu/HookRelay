package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
)

type PubSubClient struct {
	ctx     context.Context
	client  *redis.Client
	channel string
}

func NewPubsubWorker() *Worker {
	client := redis.NewClient(&redis.Options{
		Addr:     config.HRConfig.PubsubWorker.Addr,
		DB:       config.HRConfig.PubsubWorker.Db,
		Password: config.HRConfig.PubsubWorker.Password,
		Username: config.HRConfig.PubsubWorker.Username,
	})
	return &Worker{
		ID:       ulid.Make().String(),
		StopChan: make(chan struct{}),
		client: &PubSubClient{
			client:  client,
			ctx:     context.Background(),
			channel: config.HRConfig.PubsubWorker.Channel,
		},
	}
}

func (c *PubSubClient) SendJob(job *Job) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	c.client.Publish(c.ctx, c.channel, string(b))
	return nil
}

func (c *PubSubClient) ReceiveJob(jobChannel chan<- *Job) {
	// jobChannel := make(chan *Job, config.HRConfig.PubsubWorker.QueueSize) // Buffered channel for queuing jobs
	pubsub := c.client.Subscribe(c.ctx, config.HRConfig.PubsubWorker.Channel)
	for msg := range pubsub.Channel() {
		j, err := parseJobFromSubscribedChan(msg)
		if err != nil {
			continue
		}
		jobChannel <- j
	}
}

// parseJobFromMessage deserializes a Job from a Redis Pub/Sub message.
func parseJobFromSubscribedChan(msg *redis.Message) (*Job, error) {
	if msg == nil || msg.Payload == "" {
		return nil, errors.New("empty message or payload")
	}

	// Unmarshal the JSON payload into a Job struct
	var job Job
	err := json.Unmarshal([]byte(msg.Payload), &job)
	if err != nil {
		log.Printf("Failed to unmarshal job: %v", err)
		return nil, err
	}

	// Validate the job fields
	if job.Event == nil || job.Subscription == nil || job.Subscription.Target == nil {
		return nil, errors.New("invalid job: missing required fields")
	}

	return &job, nil
}

func ProcessJobFromSubscribedChan(jobChan <-chan *Job) {
	for job := range jobChan {
		processJob(job)
	}
}
