package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sermonflow-client/internal/config"
	"sermonflow-client/internal/downloader"
	"strings"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/storage"
)

type gcsNotification struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

type PubsubMessage interface {
	Ack()
	Nack()
	Data() []byte
}

type PubsubSubscriber interface {
	Receive(ctx context.Context, handler func(context.Context, PubsubMessage)) error
}

func Run(ctx context.Context, cfg *config.Config) error {
	client, err := pubsub.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return fmt.Errorf("pubsub client: %w", err)
	}
	defer client.Close()

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage client: %w", err)
	}
	defer storageClient.Close()

	logger := slog.Default()
	subscriptionPath := fmt.Sprintf("projects/%s/subscriptions/%s", cfg.ProjectID, cfg.Subscription)
	logger.Info("listening on subscription", "subscription", subscriptionPath)

	subscriber := &pubsubSubscriberAdapter{sub: client.Subscriber(subscriptionPath)}
	storageAdapter := &storageClientAdapter{storageClient}
	err = RunWithDeps(ctx, cfg, subscriber, storageAdapter)

	if err == context.Canceled {
		logger.Info("interrupted, shutting down")
		return nil
	}
	return err
}

func RunWithDeps(ctx context.Context, cfg *config.Config, subscriber PubsubSubscriber, storageClient downloader.StorageClient) error {
	return subscriber.Receive(ctx, func(innerCtx context.Context, msg PubsubMessage) {
		HandleMessageWithDeps(innerCtx, msg, storageClient, cfg)
	})
}

func handleMessage(ctx context.Context, msg *pubsub.Message, storageClient *storage.Client, cfg *config.Config) {
	adapted := &pubsubMessageAdapter{msg}
	storageAdapter := &storageClientAdapter{storageClient}
	HandleMessageWithDeps(ctx, adapted, storageAdapter, cfg)
}

func HandleMessageWithDeps(ctx context.Context, msg PubsubMessage, storageClient downloader.StorageClient, cfg *config.Config) {
	logger := slog.Default()

	var notif gcsNotification
	if err := json.Unmarshal(msg.Data(), &notif); err != nil {
		logger.Error("malformed notification payload, acking to discard", "error", err)
		msg.Ack()
		return
	}

	if !strings.HasSuffix(notif.Name, ".probundle") {
		msg.Ack()
		return
	}

	if err := downloader.Download(ctx, storageClient, cfg, notif.Name); err != nil {
		logger.Error("download failed, nacking for redelivery", "object", notif.Name, "error", err)
		msg.Nack()
		return
	}

	msg.Ack()
}

type pubsubMessageAdapter struct {
	*pubsub.Message
}

func (a *pubsubMessageAdapter) Data() []byte {
	return a.Message.Data
}

type pubsubSubscriberAdapter struct {
	sub interface {
		Receive(context.Context, func(context.Context, *pubsub.Message)) error
	}
}

func (a *pubsubSubscriberAdapter) Receive(ctx context.Context, handler func(context.Context, PubsubMessage)) error {
	return a.sub.Receive(ctx, func(innerCtx context.Context, msg *pubsub.Message) {
		handler(innerCtx, &pubsubMessageAdapter{msg})
	})
}

type storageClientAdapter struct {
	*storage.Client
}

func (a *storageClientAdapter) Bucket(name string) downloader.BucketHandle {
	return &bucketHandleAdapter{a.Client.Bucket(name)}
}

type bucketHandleAdapter struct {
	*storage.BucketHandle
}

func (a *bucketHandleAdapter) Object(name string) downloader.ObjectReader {
	return &objectHandleAdapter{a.BucketHandle.Object(name)}
}

type objectHandleAdapter struct {
	*storage.ObjectHandle
}

func (a *objectHandleAdapter) NewReader(ctx context.Context) (io.ReadCloser, error) {
	return a.ObjectHandle.NewReader(ctx)
}
