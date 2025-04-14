package publisher

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/codeasashu/HookRelay/internal/app"
	"github.com/codeasashu/HookRelay/internal/config"
	"github.com/codeasashu/HookRelay/internal/delivery"
	"github.com/codeasashu/HookRelay/internal/metrics"
	"github.com/codeasashu/HookRelay/internal/worker"
	"github.com/google/uuid"
)

// Feature defines the billing feature structure
type Feature struct {
	ResourceKey string `json:"resource_key"`
	Units       int    `json:"units"`
}

type BillingMessage struct {
	CompanyID string    `json:"company_id"`
	Features  []Feature `json:"features"`
}

type BillingPublisher struct {
	ctx           context.Context
	client        *sqs.Client
	metricsC      *metrics.Metrics
	url           string // SQS url
	batchMode     bool
	batchDuration *time.Duration
	cancelFunc    func()
}

func NewBillingPublisher(f *app.HookRelayApp, cfg *config.BillingConfig) *BillingPublisher {
	awsCfg := aws.Config{
		Region: cfg.Region,
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, ""),
		),
	}
	newCtx, cancel := context.WithCancel(f.Ctx)
	c := sqs.NewFromConfig(awsCfg)

	batchDur := time.Duration(cfg.BatchDuration) * time.Millisecond
	return &BillingPublisher{
		ctx:           newCtx,
		client:        c,
		url:           cfg.SQSUrl,
		cancelFunc:    cancel,
		batchMode:     cfg.BatchMode,
		batchDuration: &batchDur,
		metricsC:      f.Metrics,
	}
}

// AddFeature adds a feature to the message
func (b *BillingMessage) AddFeature(resourceKey string, units int) {
	b.Features = append(b.Features, Feature{
		ResourceKey: resourceKey,
		Units:       units,
	})
}

func (b *BillingMessage) AddOwnerId(ownerId string) {
	b.CompanyID = ownerId
}

func (bp *BillingPublisher) SendMessage(ctx context.Context, msg *BillingMessage) error {
	startedAt := time.Now()
	messageAttributes := map[string]types.MessageAttributeValue{
		"uid": {
			DataType:    aws.String("String"),
			StringValue: aws.String(uuid.NewString()),
		},
		"namespace": {
			DataType:    aws.String("String"),
			StringValue: aws.String("hookrelay"),
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
			StringValue: aws.String(time.Now().Format(time.RFC3339)),
		},
	}

	msgBody, err := json.Marshal(msg)
	if err != nil {
		slog.Error("sqs: failed to marshal message", "err", err)
		return err
	}
	// Send the message
	output, err := bp.client.SendMessage(bp.ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(bp.url),
		MessageBody:       aws.String(string(msgBody)),
		MessageAttributes: messageAttributes,
	})

	sourceName := worker.GetFanoutSource(ctx)
	if err != nil {
		slog.Error("failed to send message", "err", err)
		bp.metricsC.IncrementBillingErrored(sourceName)
		bp.metricsC.RecordBillingLatency(sourceName, startedAt)
		return err
	}

	bp.metricsC.IncrementBillingPublished(sourceName)
	bp.metricsC.RecordBillingLatency(sourceName, startedAt)
	slog.Info("Message sent!", "msg_id", *output.MessageId)
	return nil
}

func (bp *BillingPublisher) ProcessTasks(ctx context.Context, tasks []worker.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	companyBillingMap := make(map[string]int)

	for _, t := range tasks {
		ed := t.(*delivery.EventDelivery)
		companyBillingMap[ed.OwnerId] += 1
	}

	for ownerId, unitCnt := range companyBillingMap {
		bm := &BillingMessage{
			CompanyID: ownerId,
		}
		bm.AddFeature("webhook_incall", unitCnt)
		bp.SendMessage(ctx, bm)
	}

	sourceName := worker.GetFanoutSource(ctx)
	bp.metricsC.IncrementEventsBilled(sourceName, len(tasks))
	return nil
}

func (bp *BillingPublisher) ProcessSQSBatch(ctx context.Context, jobChan <-chan worker.Task) {
	const batchSize = 100

	var batch []worker.Task
	ticker := time.NewTicker(*bp.batchDuration)
	defer ticker.Stop()

	for {
		select {
		case task, ok := <-jobChan:
			if !ok {
				// Channel closed, flush remaining events
				if len(batch) > 0 {
					bp.ProcessTasks(ctx, batch)
				}
				return
			}

			// m.IncrementIngestConsumedTotal(job.Event, "local")
			batch = append(batch, task)

			if len(batch) >= batchSize {
				bp.ProcessTasks(ctx, batch)
				batch = nil // Reset batch
			}
		case <-bp.ctx.Done():
			return
		case <-ticker.C:
			if len(batch) > 0 {
				bp.ProcessTasks(ctx, batch)
				batch = nil // Reset batch
			}
		}
	}
}

func (bp *BillingPublisher) ProcessSQS(ctx context.Context, jobChan <-chan worker.Task) {
	for {
		select {
		case task, ok := <-jobChan:
			if !ok {
				return
			}
			bp.ProcessTasks(ctx, []worker.Task{task})
		case <-bp.ctx.Done():
			return
		}
	}
}

func (bp *BillingPublisher) SQSPublisher(ctx context.Context, f *worker.FanOut) error {
	ch, err := f.Subscribe("sqs", 1024)
	if err != nil {
		slog.Error("error starting SQS billing module", "err", err)
		return err
	}
	go func() {
		defer f.Unsubscribe("sqs")
		newCtx := worker.SetFanoutSource(f, ctx)
		if bp.batchMode {
			bp.ProcessSQSBatch(newCtx, ch)
		} else {
			bp.ProcessSQS(newCtx, ch)
		}
	}()
	return nil
}

func (bp *BillingPublisher) Shutdown() {
	bp.cancelFunc()
}
