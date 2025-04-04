package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/google/uuid"
)

// Feature defines the billing feature structure
type Feature struct {
	ResourceKey string `json:"resource_key"`
	Units       int    `json:"units"`
}

// BillingMessage defines the complete payload structure
type BillingMessage struct {
	CompanyID string    `json:"company_id"`
	Features  []Feature `json:"features"`
}

// MessageBuilder helps construct the billing message
type MessageBuilder struct {
	companyID string
	features  []Feature
}

// NewMessageBuilder initializes a builder
func NewMessageBuilder(companyID string) *MessageBuilder {
	return &MessageBuilder{
		companyID: companyID,
		features:  []Feature{},
	}
}

// AddFeature adds a feature to the message
func (b *MessageBuilder) AddFeature(resourceKey string, units int) {
	b.features = append(b.features, Feature{
		ResourceKey: resourceKey,
		Units:       units,
	})
}

// Build marshals the message into JSON
func (b *MessageBuilder) Build() ([]byte, error) {
	msg := BillingMessage{
		CompanyID: b.companyID,
		Features:  b.features,
	}
	return json.Marshal(msg)
}

func PublishEventDelivery(deliveries []worker.Task) error {
}

func main() {
	ctx := context.Background()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Create SQS client
	sqsClient := sqs.NewFromConfig(cfg)

	// Build the message
	builder := NewMessageBuilder("1")
	builder.AddFeature("webhook_incall", 1)
	// Add more features if needed:
	// builder.AddFeature("another_feature", 3)

	messageBody, err := builder.Build()
	if err != nil {
		log.Fatalf("failed to build message: %v", err)
	}

	// Prepare message attributes
	messageAttributes := map[string]sqs.MessageAttributeValue{
		"uid": {
			DataType:    aws.String("String"),
			StringValue: aws.String(uuid.NewString()),
		},
		"namespace": {
			DataType:    aws.String("String"),
			StringValue: aws.String("chat"),
		},
		"action": {
			DataType:    aws.String("String"),
			StringValue: aws.String("billing"),
		},
		"etag": {
			DataType:    aws.String("String"),
			StringValue: aws.String("Qwerty"),
		},
		"created_at": {
			DataType:    aws.String("String"),
			StringValue: aws.String("2022-09-12 11:04:49"),
		},
	}

	// Send the message
	output, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String("<QUEUE_URL>"), // Replace with your SQS Queue URL
		MessageBody:       aws.String(string(messageBody)),
		MessageAttributes: messageAttributes,
	})
	if err != nil {
		log.Fatalf("failed to send message: %v", err)
	}

	fmt.Printf("Message sent! ID: %s\n", *output.MessageId)
}
