package listener

import (
	"context"
	"encoding/json"
	"fmt"
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

// Run starts listening on a Pub/Sub subscription for GCS notifications.
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

	subscriber := client.Subscriber(subscriptionPath)
	err = subscriber.Receive(ctx, func(innerCtx context.Context, msg *pubsub.Message) {
		handleMessage(innerCtx, msg, storageClient, cfg)
	})

	if err == context.Canceled {
		logger.Info("interrupted, shutting down")
		return nil
	}
	return err
}

// handleMessage processes a single GCS notification and downloads the file if needed.
func handleMessage(ctx context.Context, msg *pubsub.Message, client *storage.Client, cfg *config.Config) {
	logger := slog.Default()

	var notif gcsNotification
	if err := json.Unmarshal(msg.Data, &notif); err != nil {
		logger.Error("malformed notification payload, acking to discard", "error", err)
		msg.Ack()
		return
	}

	// Only process .probundle files
	if !strings.HasSuffix(notif.Name, ".probundle") {
		msg.Ack()
		return
	}

	if err := downloader.Download(ctx, client, cfg, notif.Name); err != nil {
		logger.Error("download failed, nacking for redelivery", "object", notif.Name, "error", err)
		msg.Nack()
		return
	}

	msg.Ack()
}
