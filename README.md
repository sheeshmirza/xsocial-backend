# xsocial-backend

A high-performance social media management backend written in Go, fully migrated from TypeScript (`xsocial-backend-legacy`).

## Features

- **Dedicated Platform Modules**:
  - **Instagram**:
    - Dedicated OAuth adapter with long-lived token exchange (`fb_exchange_token`) & token refresh.
    - Dedicated Publish adapter supporting single images, single videos (Reels), and multi-item carousels (images/videos) with asynchronous container publishing and status polling.
    - Dedicated Service, Handler, and Models.
  - **LinkedIn**:
    - Dedicated OAuth adapter with code-for-token exchange & userinfo lookup.
    - Dedicated Publish adapter supporting text-only posts, single/multiple image posts, video uploads (REST protocol initialization, upload part, finish), and multi-image carousels.
    - Dedicated Activity adapter for fetching member / organization posts with graceful scope handling.
    - Dedicated Service, Handler, and Models.
- **Unified Post Schema & State Machine**:
  - Unified data model in MongoDB supporting all post states:
    - `draft`
    - `scheduled` (supports `once` and `recurring` cadence: daily, weekly, monthly, yearly with month-end date clamping)
    - `posted` / `published`
    - `failed`
  - Target-specific per-platform metadata (`platformPostId`, `platformPostUrl`, error tracking).
  - Bulk actions, pause/resume recurrence, date/timezone scheduling.
- **Authentication & Security**:
  - JWT authentication with access tokens and rotating refresh tokens.
  - SHA-256 hashed refresh tokens stored in MongoDB with expiration.
  - AES-256-GCM encryption for OAuth tokens at rest.
  - Password hashing via `bcrypt`.
- **Background Jobs & Worker Queue**:
  - Built-in Redis-backed task queue with concurrent worker pools.
  - Automated job processing:
    - Post publishing worker (`queue:publish_post`)
    - Recurring post scheduler
    - Token refresher (`queue:token_refresh`)
    - Expired media cleanup (`queue:cleanup_media`)
    - LinkedIn activity sync
- **Media Uploads**:
  - In-memory upload buffer with TTL expiry and base64 preview generation.
- **AI Post Generation**:
  - Integration with Gemini API for automated caption and hashtag generation.

---

## Architecture Overview

```
xsocial-backend/
├── cmd/
│   └── server/
│       └── main.go                 # Application entrypoint & worker bootstrap
├── internal/
│   ├── config/                     # Configuration loading & validation
│   ├── database/                   # MongoDB v2 driver & Redis client setup
│   ├── models/                     # Shared domain models (User, Post, RefreshToken, etc.)
│   ├── repository/                 # MongoDB repository layer
│   ├── platform/                   # Dedicated platform modules
│   │   ├── common/                 # Reusable HTTP client and platform interfaces
│   │   ├── instagram/              # Instagram adapters (OAuth, Publish), handler, service, models
│   │   └── linkedin/               # LinkedIn adapters (OAuth, Publish, Activity), handler, service, models
│   ├── service/                    # Business logic (Auth, Users, Posts, Uploads, AI, Queue)
│   ├── handler/                    # Gin HTTP request handlers
│   ├── middleware/                 # Auth, CORS, and centralized error recovery middleware
│   ├── queue/                      # Redis job queue & worker engine
│   ├── router/                     # Gin route definitions
│   └── utils/                      # Pagination, crypto, error formatting, account filters
├── Makefile                        # Build and development commands
└── go.mod                          # Go module dependencies
```

---

## Getting Started

### Prerequisites

- **Go 1.24+**
- **MongoDB** (running locally or MongoDB Atlas)
- **Redis** (running locally or Redis Cloud)

### Configuration

Copy `.env.example` to `.env` and fill in your credentials:

```bash
cp .env.example .env
```

Key environment variables:
- `PORT`: Server port (default `8000`)
- `MONGODB_URI`: MongoDB connection string
- `DB_NAME`: Database name (e.g. `xsocial`)
- `REDIS_URL`: Redis URI (e.g. `redis://localhost:6379`)
- `JWT_ACCESS_SECRET` / `JWT_REFRESH_SECRET`: Secrets for signing JWTs
- `ENCRYPTION_KEY`: 32-byte secret key for AES-256 token encryption
- `INSTAGRAM_CLIENT_ID` / `INSTAGRAM_CLIENT_SECRET`: Meta App credentials
- `LINKEDIN_CLIENT_ID` / `LINKEDIN_CLIENT_SECRET`: LinkedIn Developer App credentials
- `GEMINI_API_KEY`: Google Gemini API key

### Build & Run

To build the executable:
```bash
make build
```

To run the server:
```bash
make run
```

Or directly via Go:
```bash
go run ./cmd/server
```
