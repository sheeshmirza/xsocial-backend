# xSocial — Enterprise Social Media Management & MCP AI Agent Orchestration Platform

[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go)](https://golang.org)
[![Next.js](https://img.shields.io/badge/Next.js-14.2-black?style=flat&logo=next.js)](https://nextjs.org)
[![MongoDB](https://img.shields.io/badge/MongoDB-Driver_v2-47A248?style=flat&logo=mongodb)](https://mongodb.com)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-ACID_3NF-336791?style=flat&logo=postgresql)](https://postgresql.org)
[![Redis](https://img.shields.io/badge/Redis-Multi--Tier_Cache-DC382D?style=flat&logo=redis)](https://redis.io)
[![RabbitMQ](https://img.shields.io/badge/RabbitMQ-AMQP_0.9.1-FF6600?style=flat&logo=rabbitmq)](https://rabbitmq.com)
[![Qdrant](https://img.shields.io/badge/Qdrant-Vector_Search-DC2626?style=flat&logo=qdrant)](https://qdrant.tech)

xSocial is a production-grade, enterprise social media planning, publishing, and analytics platform. Built with **Go** and **Next.js 14**, it provides high-throughput social automation across Instagram and LinkedIn, multi-workspace role-based access control (RBAC), distributed queueing, multi-tier caching, and a built-in **Model Context Protocol (MCP 2025-06-18)** JSON-RPC 2.0 server with OAuth 2.1 discovery for autonomous AI agents.

---

## 🏛️ System Architecture

```
                                 ┌────────────────────────────────────────────────────────┐
                                 │                   Clients & Frontends                  │
                                 │  Next.js 14 App Router, Web Apps, MCP AI Agents (LLMs) │
                                 └───────────────────────────┬────────────────────────────┘
                                                             │ HTTPS / REST / JSON-RPC
                                                             ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Presentation & Ingress Layer (`internal/api`)                                                                           │
│  • Unified HTTP Route Multiplexer (`/v1/*`, `/api/v1/*`, `/*` routing aliases)                                          │
│  • W3C `traceparent` & `X-Request-ID` automated propagation                                                              │
│  • Token Bucket Rate Limiting (IP + Account level) & Brute-Force Shielding (bounded memory pruning)                      │
│  • Model Context Protocol (MCP 2025-06-18) Engine with OAuth 2.1 Metadata Discovery                                      │
│  • Uniform Error Response Envelopes & Redacting Logger (zero topology/credential leakage)                               │
└─────────────────────────────────────────────┬────────────────────────────────────────────────────────────────────────────┘
                                              │ Injects Domain Services
                                              ▼
┌──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Application & Domain Service Layer (`internal/service`)                                                                  │
│  • AuthService, UserService, PostService, PostPublishService, BrandService, ApiKeyService                                │
│  • VectorSearchService (Dense 128-dim embeddings, semantic post discovery)                                               │
│  • Safe Goroutine Execution (`utils.SafeGo` with panic recovery & stack trace logging)                                   │
│  • Defensive Nil Guards across all external dependencies and multi-tenant workspaces                                     │
└──────────────┬───────────────────────────────┬───────────────────────────────┬────────────────────────────┬──────────────┘
               │                               │                               │                            │
               ▼                               ▼                               ▼                            ▼
┌──────────────────────────────┐┌──────────────────────────────┐┌──────────────────────────────┐┌──────────────────────────┐
│ Persistence Engines          ││ Distributed Caching          ││ Asynchronous Message Broker  ││ Vector Database Engine   │
│ (`internal/database`,        ││ (`internal/database/cache`)  ││ (`internal/queue`)           ││ (`internal/database`)    │
│  `internal/repository`)      ││ • L1: Local In-Memory Cache  ││ • RabbitMQ AMQP 0.9.1 Broker ││ • Qdrant Vector Client   │
│ • MongoDB v2 (Pool: 300/25)  ││ • L2: Redis Cluster          ││   - Direct Exchange:         ││ • Cosine similarity     │
│ • PostgreSQL Relational      ││ • Sub-millisecond reads      ││     `xsocial.direct`         ││ • Automatic collection   │
│   (Pool: 100/25, ACID tx)    ││ • Write-through invalidation ││   - Queue: `posts.publish`   ││   bootstrapping & search │
│                              ││                              ││   - DLQ: `xsocial.posts.dlq` ││ • Lifecycle post index  │
└──────────────────────────────┘└──────────────────────────────┘└──────────────────────────────┘└──────────────────────────┘
```

---

## 🚀 Key Capabilities

### 1. Multi-Engine Persistence & Caching
- **MongoDB**: Primary document storage for users, brand workspaces, social handles, posts, activities, and audit logs with connection pooling (`MaxPoolSize: 300`, `MinPoolSize: 25`).
- **PostgreSQL**: Relational database engine with high-throughput pooling (`SetMaxOpenConns: 100`, `SetMaxIdleConns: 25`) and ACID transactions with automatic rollback on errors or panics.
- **Redis Multi-Tier Cache**: L1 in-memory sharded micro-cache + L2 distributed Redis cache with automated write-through invalidation, sub-millisecond retrieval, and token revocation lists.
- **RabbitMQ Message Broker**: Asynchronous post publishing engine with durable direct exchanges (`xsocial.direct`), durable queues (`xsocial.posts.publish`), consumer prefetch QoS, Dead-Letter Exchange (`xsocial.dlx`), and W3C trace context preservation.
- **Qdrant Vector Database**: Dense vector indexing and similarity search (`GET /v1/posts/search/semantic`), with automatic lifecycle indexing on post creation, update, and deletion.

### 2. Security & Zero-Leakage Architecture
- **Distributed Tracing**: Full W3C `traceparent` (`00-<trace_id>-<span_id>-<flags>`) and `X-Request-ID` extraction, derivation, and propagation.
- **Sensitive Data Masking**: Structured logging automatically redacts connection URLs, passwords, Bearer tokens, and API keys (`xs_live_`, `mc_live_`).
- **Anti-Brute-Force Protection**: IP and account rate limiting with automatic background pruning to bound memory utilization.
- **Strict SSRF Mitigation**: Outbound media downloads and webhooks block RFC-1918 private IPs, loopback interfaces (`127.0.0.1`), and cloud metadata IP (`169.254.169.254`).

### 3. Model Context Protocol (MCP 2025-06-18)
- Exposes standard JSON-RPC 2.0 endpoints on `/mcp`.
- Supports tools discovery (`tools/list`), execution (`tools/call`), and OAuth 2.1 resource metadata discovery (`/.well-known/oauth-protected-resource`).

---

## 🛠️ Getting Started

### Prerequisites
- **Go**: Version 1.26 or higher
- **Node.js**: Version 18.x or higher & npm
- **MongoDB**: v6.0+ (standalone or cluster)
- **Redis**: v7.0+
- **PostgreSQL**: v14+ (optional)
- **RabbitMQ**: v3.12+ (optional)
- **Qdrant**: v1.7+ (optional)

---

### Backend Setup (`xsocial-backend`)

1. **Configure Environment Variables**:
   ```bash
   cp .env.example .env
   ```
   Open `.env` and verify your settings. In production, ensure `JWT_ACCESS_SECRET` and `JWT_REFRESH_SECRET` are >= 32 random characters:
   ```bash
   openssl rand -hex 32
   ```

2. **Run Tests**:
   ```bash
   go test -count=1 ./...
   ```

3. **Start the API Server**:
   ```bash
   go run main.go
   ```
   The backend server starts on port `5000` (default) with REST endpoints mounted at `/api/v1` and MCP at `/mcp`.

---

### Frontend Setup (`xsocial-frontend`)

1. **Navigate to Frontend Directory**:
   ```bash
   cd ../xsocial-frontend
   ```

2. **Configure Environment Variables**:
   ```bash
   cp .env.example .env.local
   ```

3. **Install Dependencies & Verify Build**:
   ```bash
   npm run lint        # Verifies ESLint Core Web Vitals (0 errors, 0 warnings)
   npm run type-check   # Verifies strict TypeScript compliance (0 errors)
   npm run build       # Creates optimized production build (18/18 static pages)
   ```

4. **Start Development Server**:
   ```bash
   npm run start:dev
   ```
   Open [http://localhost:3000](http://localhost:3000) in your browser.

---

## 📖 API Reference

### Health & Readiness
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/health` / `/healthz` | System health check (MongoDB, Redis, PostgreSQL, RabbitMQ, Qdrant) |
| `GET` | `/health?ready=1` | Kubernetes readiness probe (returns 503 if any required component is degraded) |

### Authentication & Identity
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/v1/auth/signup` | Register new user account |
| `POST` | `/v1/auth/login` | Authenticate user & receive JWT Access/Refresh tokens |
| `POST` | `/v1/auth/refresh` | Rotate expired Access Token using Refresh Token |
| `POST` | `/v1/auth/logout` | Invalidate active Refresh Token |
| `GET` | `/v1/users/me` | Fetch authenticated user profile |
| `PUT` | `/v1/users/me` | Update profile information |

### Workspaces & Brands (Multi-Tenant RBAC)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/v1/brands` | List accessible brand workspaces |
| `POST` | `/v1/brands` | Create brand workspace |
| `GET` | `/v1/brands/{id}` | Retrieve brand workspace details |
| `POST` | `/v1/brands/{id}/members` | Add team member with role |

### Social Accounts & OAuth
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/v1/social/accounts` | List connected social profiles |
| `POST` | `/v1/social/connect/{platform}` | Initiate OAuth handshake (Instagram, LinkedIn) |
| `DELETE` | `/v1/social/{id}` | Disconnect social media account |

### Posts & Semantic Vector Search
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/v1/posts` | Create new draft or scheduled post (auto-indexed into Qdrant) |
| `GET` | `/v1/posts` | List posts with pagination, sorting, and filters |
| `GET` | `/v1/posts/search/semantic` | Query post semantic similarity via Qdrant vector embeddings |
| `PUT` | `/v1/posts/{id}` | Update existing post |
| `DELETE` | `/v1/posts/{id}` | Delete post (purges vector from Qdrant) |
| `POST` | `/v1/posts/{id}/publish` | Instantly dispatch post for publishing |

### Model Context Protocol (MCP)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/mcp` | MCP JSON-RPC 2.0 endpoint (`tools/list`, `tools/call`) |
| `GET` | `/.well-known/oauth-protected-resource` | RFC 9207 / MCP OAuth 2.1 protected resource metadata |

---

## 🔒 Security Best Practices

1. **Keep Secrets Safe**: Never commit `.env` or `.env.local` files to source control.
2. **Rotate Tokens**: Use cryptographically signed JWT tokens with short expiration windows (`15m` access, `30d` refresh).
3. **Audit Trails**: All state mutations (post creation, updates, deletes, member invitations) are written to immutable audit logs.
4. **Resilient Workers**: All asynchronous background workers run through panic-shielded goroutines (`utils.SafeGo`).
