# GCP Setup — SermonFlow Client

All commands to provision the GCP resources the client depends on.
Run these once from your dev machine with `gcloud` authenticated as the project owner.

## Prerequisites

```bash
# confirm active project
gcloud config list

# set if needed
gcloud config set project project-1a6a8036-8e9b-4c44-98e
gcloud config set run/region us-central1
```

## Enable required APIs

```bash
gcloud services enable \
  pubsub.googleapis.com \
  storage.googleapis.com
```

---

## Pub/Sub

### Create the topic

```bash
gcloud pubsub topics create sermonflow-output-notifications
```

### Create the subscription

```bash
gcloud pubsub subscriptions create sermonflow-slide-computer \
  --topic=sermonflow-output-notifications \
  --ack-deadline=60 \
  --message-retention-duration=7d
```

`--ack-deadline=60` gives the client 60 seconds to download the file and ack the
message before Pub/Sub redelivers it. Increase if downloads are slow.

`--message-retention-duration=7d` means if the slide computer is offline for up to
a week, messages wait and are delivered on reconnect.

### Attach GCS notifications to the topic

This tells GCS to publish a message every time a file is finalized in `output/`.

```bash
# get the GCS service agent for this project
GCS_SA=$(gcloud storage service-agent --project=project-1a6a8036-8e9b-4c44-98e)

# grant it permission to publish to the topic
gcloud pubsub topics add-iam-policy-binding sermonflow-output-notifications \
  --member="serviceAccount:$GCS_SA" \
  --role="roles/pubsub.publisher"

# create the bucket notification
gcloud storage buckets notifications create gs://sermonflow-io \
  --topic=sermonflow-output-notifications \
  --event-types=OBJECT_FINALIZE \
  --object-prefix=output/
```

`OBJECT_FINALIZE` fires only when an upload is fully complete — not mid-stream.
`--object-prefix=output/` scopes it so uploads to `in/` don't generate noise.

### Verify notifications are configured

```bash
gcloud storage buckets notifications list gs://sermonflow-io
```

---

## Service Account for the Slide Computer

### Create the service account

```bash
gcloud iam service-accounts create sermonflow-client-sa \
  --display-name="SermonFlow Slide Computer"
```

### Grant Pub/Sub pull access

```bash
gcloud pubsub subscriptions add-iam-policy-binding sermonflow-slide-computer \
  --member="serviceAccount:sermonflow-client-sa@project-1a6a8036-8e9b-4c44-98e.iam.gserviceaccount.com" \
  --role="roles/pubsub.subscriber"
```

### Grant GCS read access

```bash
gcloud storage buckets add-iam-policy-binding gs://sermonflow-io \
  --member="serviceAccount:sermonflow-client-sa@project-1a6a8036-8e9b-4c44-98e.iam.gserviceaccount.com" \
  --role="roles/storage.objectViewer"
```

### Generate a key file for the slide computer

```bash
gcloud iam service-accounts keys create ~/sermonflow-client-key.json \
  --iam-account=sermonflow-client-sa@project-1a6a8036-8e9b-4c44-98e.iam.gserviceaccount.com
```

Transfer `sermonflow-client-key.json` to the slide computer securely (AirDrop, USB).
Set `GOOGLE_APPLICATION_CREDENTIALS` in `.env` to its absolute path on that machine.
Do not commit this file anywhere.

---

## Verification

### Check topic exists

```bash
gcloud pubsub topics list
```

### Check subscription exists and is attached to the right topic

```bash
gcloud pubsub subscriptions describe sermonflow-slide-computer
```

### Check IAM on the subscription

```bash
gcloud pubsub subscriptions get-iam-policy sermonflow-slide-computer
```

### Check IAM on the bucket

```bash
gcloud storage buckets get-iam-policy gs://sermonflow-io
```

### Manually publish a test message to the topic

Useful for testing the client without running the full encode job:

```bash
gcloud pubsub topics publish sermonflow-output-notifications \
  --message='{"bucket":"sermonflow-io","name":"output/test.probundle"}'
```

---

## Key rotation / revocation

```bash
# list all keys for the service account
gcloud iam service-accounts keys list \
  --iam-account=sermonflow-client-sa@project-1a6a8036-8e9b-4c44-98e.iam.gserviceaccount.com

# revoke a specific key
gcloud iam service-accounts keys delete KEY_ID \
  --iam-account=sermonflow-client-sa@project-1a6a8036-8e9b-4c44-98e.iam.gserviceaccount.com

# generate a new key for the replacement machine
gcloud iam service-accounts keys create ~/sermonflow-client-key-new.json \
  --iam-account=sermonflow-client-sa@project-1a6a8036-8e9b-4c44-98e.iam.gserviceaccount.com
```

---

## Teardown (if needed)

```bash
gcloud storage buckets notifications delete gs://sermonflow-io --all
gcloud pubsub subscriptions delete sermonflow-slide-computer
gcloud pubsub topics delete sermonflow-output-notifications
gcloud iam service-accounts delete \
  sermonflow-client-sa@project-1a6a8036-8e9b-4c44-98e.iam.gserviceaccount.com
```
