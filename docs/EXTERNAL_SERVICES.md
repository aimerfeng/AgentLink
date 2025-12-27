# External Services Configuration Guide

This document provides instructions for setting up the external services required by AgentLink.

## Table of Contents

1. [Payment Services](#payment-services)
   - [Stripe](#stripe)
   - [Coinbase Commerce](#coinbase-commerce)
2. [AI Providers](#ai-providers)
   - [OpenAI](#openai)
   - [Anthropic (Claude)](#anthropic-claude)
   - [Google AI (Gemini)](#google-ai-gemini)
3. [Object Storage](#object-storage)
   - [AWS S3](#aws-s3)
   - [MinIO (Local Development)](#minio-local-development)
4. [Email Service](#email-service)
   - [SendGrid](#sendgrid)
   - [Resend](#resend)
   - [Mailhog (Local Development)](#mailhog-local-development)
5. [Blockchain](#blockchain)
   - [Alchemy (RPC Provider)](#alchemy-rpc-provider)

---

## Payment Services

### Stripe

Stripe is used for fiat currency payments.

#### Setup Steps

1. **Create Account**
   - Go to [https://dashboard.stripe.com/register](https://dashboard.stripe.com/register)
   - Complete the registration process

2. **Get API Keys**
   - Navigate to Developers → API Keys
   - Copy the **Secret key** (starts with `sk_test_` for test mode)
   - Copy the **Publishable key** (starts with `pk_test_` for test mode)

3. **Configure Webhook**
   - Navigate to Developers → Webhooks
   - Click "Add endpoint"
   - Enter your webhook URL: `https://your-domain.com/api/v1/payments/webhook/stripe`
   - Select events to listen for:
     - `checkout.session.completed`
     - `payment_intent.succeeded`
     - `payment_intent.payment_failed`
   - Copy the **Webhook signing secret** (starts with `whsec_`)

4. **Update Environment Variables**
   ```bash
   STRIPE_SECRET_KEY=sk_test_xxx
   STRIPE_PUBLISHABLE_KEY=pk_test_xxx
   STRIPE_WEBHOOK_SECRET=whsec_xxx
   ```

#### Test Mode vs Live Mode

- Use **Test Mode** keys during development (keys start with `sk_test_` and `pk_test_`)
- Switch to **Live Mode** keys for production (keys start with `sk_live_` and `pk_live_`)
- Test card numbers: [Stripe Testing Documentation](https://stripe.com/docs/testing)

---

### Coinbase Commerce

Coinbase Commerce is used for cryptocurrency payments.

#### Setup Steps

1. **Create Account**
   - Go to [https://commerce.coinbase.com/](https://commerce.coinbase.com/)
   - Sign up with your Coinbase account or create a new one

2. **Get API Key**
   - Navigate to Settings → API Keys
   - Create a new API key
   - Copy the API key

3. **Configure Webhook**
   - Navigate to Settings → Webhook subscriptions
   - Add your webhook URL: `https://your-domain.com/api/v1/payments/webhook/coinbase`
   - Copy the **Shared Secret**

4. **Update Environment Variables**
   ```bash
   COINBASE_API_KEY=xxx
   COINBASE_WEBHOOK_SECRET=xxx
   ```

---

## AI Providers

### OpenAI

#### Setup Steps

1. **Create Account**
   - Go to [https://platform.openai.com/signup](https://platform.openai.com/signup)

2. **Get API Key**
   - Navigate to API Keys section
   - Click "Create new secret key"
   - Copy the key (starts with `sk-`)

3. **Update Environment Variables**
   ```bash
   OPENAI_API_KEY=sk-xxx
   OPENAI_ORG_ID=org-xxx  # Optional
   ```

#### Supported Models
- GPT-4 Turbo (`gpt-4-turbo-preview`)
- GPT-4 (`gpt-4`)
- GPT-3.5 Turbo (`gpt-3.5-turbo`)

---

### Anthropic (Claude)

#### Setup Steps

1. **Create Account**
   - Go to [https://console.anthropic.com/](https://console.anthropic.com/)

2. **Get API Key**
   - Navigate to API Keys
   - Create a new key
   - Copy the key (starts with `sk-ant-`)

3. **Update Environment Variables**
   ```bash
   ANTHROPIC_API_KEY=sk-ant-xxx
   ```

#### Supported Models
- Claude 3 Opus (`claude-3-opus-20240229`)
- Claude 3 Sonnet (`claude-3-sonnet-20240229`)
- Claude 3 Haiku (`claude-3-haiku-20240307`)

---

### Google AI (Gemini)

#### Setup Steps

1. **Create Project**
   - Go to [https://console.cloud.google.com/](https://console.cloud.google.com/)
   - Create a new project or select existing one

2. **Enable API**
   - Navigate to APIs & Services → Library
   - Search for "Generative Language API"
   - Enable the API

3. **Get API Key**
   - Navigate to APIs & Services → Credentials
   - Create an API key
   - Copy the key

4. **Update Environment Variables**
   ```bash
   GOOGLE_AI_API_KEY=xxx
   ```

#### Supported Models
- Gemini Pro (`gemini-pro`)
- Gemini Pro Vision (`gemini-pro-vision`)

---

## Object Storage

### AWS S3

For production deployments.

#### Setup Steps

1. **Create S3 Bucket**
   - Go to AWS Console → S3
   - Create a new bucket (e.g., `agentlink-files`)
   - Configure CORS if needed

2. **Create IAM User**
   - Go to IAM → Users → Create user
   - Attach policy with S3 permissions
   - Create access keys

3. **Update Environment Variables**
   ```bash
   S3_ENDPOINT=https://s3.amazonaws.com
   S3_BUCKET=agentlink-files
   S3_REGION=us-east-1
   AWS_ACCESS_KEY_ID=xxx
   AWS_SECRET_ACCESS_KEY=xxx
   S3_USE_PATH_STYLE=false
   ```

---

### MinIO (Local Development)

MinIO provides S3-compatible storage for local development.

#### Setup

MinIO is included in the `docker-compose.yml` file. No additional setup required.

#### Default Credentials
```bash
S3_ENDPOINT=http://localhost:9000
AWS_ACCESS_KEY_ID=minioadmin
AWS_SECRET_ACCESS_KEY=minioadmin
```

#### Access MinIO Console
- URL: http://localhost:9001
- Username: minioadmin
- Password: minioadmin

---

## Email Service

### SendGrid

#### Setup Steps

1. **Create Account**
   - Go to [https://sendgrid.com/](https://sendgrid.com/)

2. **Create API Key**
   - Navigate to Settings → API Keys
   - Create a new API key with "Mail Send" permission
   - Copy the key (starts with `SG.`)

3. **Verify Sender**
   - Navigate to Settings → Sender Authentication
   - Verify your sending domain or email

4. **Update Environment Variables**
   ```bash
   EMAIL_PROVIDER=sendgrid
   SENDGRID_API_KEY=SG.xxx
   SMTP_FROM_EMAIL=noreply@yourdomain.com
   ```

---

### Resend

#### Setup Steps

1. **Create Account**
   - Go to [https://resend.com/](https://resend.com/)

2. **Get API Key**
   - Navigate to API Keys
   - Create a new key
   - Copy the key (starts with `re_`)

3. **Update Environment Variables**
   ```bash
   EMAIL_PROVIDER=resend
   RESEND_API_KEY=re_xxx
   SMTP_FROM_EMAIL=noreply@yourdomain.com
   ```

---

### Mailhog (Local Development)

Mailhog is included in `docker-compose.yml` for local email testing.

#### Access
- SMTP: localhost:1025
- Web UI: http://localhost:8025

#### Configuration
```bash
EMAIL_PROVIDER=smtp
SMTP_HOST=localhost
SMTP_PORT=1025
```

---

## Blockchain

### Alchemy (RPC Provider)

Alchemy provides reliable RPC endpoints for blockchain interactions.

#### Setup Steps

1. **Create Account**
   - Go to [https://www.alchemy.com/](https://www.alchemy.com/)

2. **Create App**
   - Create a new app
   - Select network: Base Sepolia (testnet) or Base Mainnet

3. **Get RPC URL**
   - Copy the HTTPS URL from your app dashboard

4. **Update Environment Variables**
   ```bash
   BLOCKCHAIN_NETWORK=base-sepolia
   BLOCKCHAIN_RPC_URL=https://base-sepolia.g.alchemy.com/v2/your-api-key
   ```

#### Wallet Setup

For development, you'll need a wallet with testnet ETH:

1. Create a new wallet (e.g., using MetaMask)
2. Export the private key
3. Get testnet ETH from a faucet:
   - Base Sepolia: [https://www.alchemy.com/faucets/base-sepolia](https://www.alchemy.com/faucets/base-sepolia)

```bash
BLOCKCHAIN_PRIVATE_KEY=your-private-key-without-0x-prefix
```

⚠️ **Security Warning**: Never use a wallet with real funds for development. Always use a dedicated development wallet.

---

## Quick Reference

| Service | Environment Variable | Where to Get |
|---------|---------------------|--------------|
| Stripe | `STRIPE_SECRET_KEY` | [Stripe Dashboard](https://dashboard.stripe.com/apikeys) |
| Coinbase | `COINBASE_API_KEY` | [Coinbase Commerce](https://commerce.coinbase.com/settings/api-keys) |
| OpenAI | `OPENAI_API_KEY` | [OpenAI Platform](https://platform.openai.com/api-keys) |
| Anthropic | `ANTHROPIC_API_KEY` | [Anthropic Console](https://console.anthropic.com/) |
| Google AI | `GOOGLE_AI_API_KEY` | [Google Cloud Console](https://console.cloud.google.com/apis/credentials) |
| SendGrid | `SENDGRID_API_KEY` | [SendGrid Settings](https://app.sendgrid.com/settings/api_keys) |
| Alchemy | `BLOCKCHAIN_RPC_URL` | [Alchemy Dashboard](https://dashboard.alchemy.com/) |
