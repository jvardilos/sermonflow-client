# GCP Secret Manager Setup for Shared Machines

This guide shows how to securely store configuration in GCP Secret Manager instead of local `.env` files.

## Why Secret Manager on Shared Machines?

- ✅ **No credentials on disk** - Others can't read `.env` files
- ✅ **Access control** - Only users with GCP IAM permissions can read secrets
- ✅ **Audit trail** - Every access is logged
- ✅ **Easy rotation** - Change secrets without redeploying binary
- ✅ **Encryption at rest** - GCP handles encryption

---

## Setup Steps

### 1. Create a GCP Service Account (One-time)

```bash
# Set your project
export GCP_PROJECT_ID="your-project-id"

# Create service account for sermonflow-client
gcloud iam service-accounts create sermonflow-client \
  --display-name="Sermonflow Client Service Account"

# Get the email
export SERVICE_ACCOUNT="sermonflow-client@${GCP_PROJECT_ID}.iam.gserviceaccount.com"
```

### 2. Grant Permissions to the Service Account

```bash
# Allow access to Secret Manager
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role="roles/secretmanager.secretAccessor"

# Allow access to Pub/Sub and GCS (existing permissions)
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role="roles/pubsub.subscriber"

gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role="roles/storage.objectViewer"
```

### 3. Create a Service Account Key (for local machine)

```bash
# Create and download key
gcloud iam service-accounts keys create /tmp/sermonflow-key.json \
  --iam-account=$SERVICE_ACCOUNT

# Move to secure location
mkdir -p ~/.config/sermonflow
mv /tmp/sermonflow-key.json ~/.config/sermonflow/creds.json
chmod 600 ~/.config/sermonflow/creds.json
```

### 4. Create Secrets in Secret Manager

```bash
# Store each secret
echo -n "my-project-id" | gcloud secrets create gcp-project-id \
  --data-file=- \
  --replication-policy="automatic"

echo -n "my-subscription" | gcloud secrets create pubsub-subscription \
  --data-file=- \
  --replication-policy="automatic"

echo -n "my-bucket" | gcloud secrets create gcs-bucket \
  --data-file=- \
  --replication-policy="automatic"

echo -n "/path/to/workspace" | gcloud secrets create workspace-dir \
  --data-file=- \
  --replication-policy="automatic"

# For credentials path (optional, or just use env var)
echo -n "~/.config/sermonflow/creds.json" | gcloud secrets create gcp-credentials-path \
  --data-file=- \
  --replication-policy="automatic"
```

### 5. Set Up Local Environment

```bash
# Create .env file for local development (env vars take precedence)
cat > ~/.config/sermonflow/.env << 'EOF'
GOOGLE_APPLICATION_CREDENTIALS=~/.config/sermonflow/creds.json
GCP_PROJECT_ID=your-project-id
PUBSUB_SUBSCRIPTION=your-subscription
GCS_BUCKET=your-bucket
PROPRESENTER_WORKSPACE_DIR=/path/to/workspace
EOF

# Lock down permissions
chmod 600 ~/.config/sermonflow/.env

# Add to shell profile (~/.zshrc or ~/.bashrc)
export SERMONFLOW_CONFIG=~/.config/sermonflow
source $SERMONFLOW_CONFIG/.env
```

### 6. Run the Application

The application will:
1. **First check environment variables** (from `.env` file)
2. **Fall back to Secret Manager** if env var not found

```bash
# If .env is loaded, it uses env vars (fast, local)
./bin/sermonflow-client

# If running on different machine without .env, 
# it automatically fetches from Secret Manager
./bin/sermonflow-client
# (requires valid GOOGLE_APPLICATION_CREDENTIALS)
```

---

## How It Works (Code)

```go
// getConfigValue checks env var first, then Secret Manager
func getConfigValue(ctx context.Context, envVar, secretName string) string {
    // 1. Try environment variable (from .env)
    if val := os.Getenv(envVar); val != "" {
        return val  // ← Returns immediately if found
    }

    // 2. Fall back to Secret Manager
    if val, err := getSecretValue(ctx, secretName); err == nil {
        return val  // ← Fetches from GCP if env var missing
    }

    return ""
}
```

**Why this is secure on shared machines:**
- User A's `.env` is only readable by User A (`chmod 600`)
- If another user runs the app without their own `.env`, it fetches from Secret Manager
- All fetches are logged in GCP Cloud Audit Logs
- Credentials are never written to disk by the app

---

## Audit Trail

Check who accessed secrets:

```bash
# View audit logs
gcloud logging read 'resource.type="secretmanager.googleapis.com"' \
  --limit=50 \
  --format=json

# Filter for specific secret
gcloud logging read 'resource.type="secretmanager.googleapis.com" 
  AND protoPayload.resourceName:gcp-project-id' \
  --format='table(timestamp,protoPayload.authenticationInfo.principalEmail,protoPayload.methodName)'
```

You'll see:
- **Who** accessed the secret (service account email)
- **When** it was accessed (timestamp)
- **What** operation was performed (AccessSecretVersion)
- **Success/failure** status

---

## Updating Secrets

```bash
# Update a secret
echo -n "new-value" | gcloud secrets versions add pubsub-subscription \
  --data-file=-

# Old secrets are kept as versions; you can revert if needed
gcloud secrets versions list pubsub-subscription
```

The app fetches `versions/latest`, so it picks up new values immediately on next run.

---

## Deployment Options

### **Cloud Run** (Recommended)
```bash
gcloud run deploy sermonflow-client \
  --image gcr.io/myproject/sermonflow-client \
  --service-account=$SERVICE_ACCOUNT \
  --update-secrets PUBSUB_SUBSCRIPTION=pubsub-subscription:latest
```

### **GKE**
```yaml
# Create Kubernetes Secret from GCP Secret Manager
kubectl create secret generic sermonflow-secrets \
  --from-literal=pubsub-subscription=$(gcloud secrets versions access latest --secret="pubsub-subscription")
```

### **Self-hosted / systemd**
```ini
# /etc/systemd/system/sermonflow.service
[Service]
Environment="GOOGLE_APPLICATION_CREDENTIALS=/etc/sermonflow/creds.json"
Environment="GCP_PROJECT_ID=my-project"
# App will fetch remaining secrets from Secret Manager
```

---

## Security Comparison

| Method | Disk | Process List | Audit Log | Rotation |
|--------|------|--------------|-----------|----------|
| `.env` file | ❌ Visible | ❌ Visible | ❌ No | ❌ Slow |
| Secret Manager | ✅ No | ✅ No | ✅ Yes | ✅ Fast |
| Env vars only | ❌ Visible | ❌ Visible | ❌ No | ❌ Slow |

---

## Testing

```bash
# Test with env vars
export GCP_PROJECT_ID="test-project"
./bin/sermonflow-client

# Test with Secret Manager (remove env vars first)
unset GCP_PROJECT_ID
unset PUBSUB_SUBSCRIPTION
./bin/sermonflow-client  # Will fetch from Secret Manager
```
