# Metricool / XSocial Backend

A high-performance all-in-one social media management, analytics, scheduling, inbox, and competitor benchmarking platform written in Go, incorporating all features of **Metricool.com** with **API Key Access** and a **Model Context Protocol (MCP)** server.

Built with clean architecture patterns referencing the battle-tested design of `mailhost-backend`.

---

## Metricool Feature Suite

### 1. Multi-Brand & Workspace Management (Agencies & Teams)
- Multi-brand architecture supporting agencies managing hundreds of clients.
- Granular Role-Based Access Control (RBAC): `Owner`, `Admin`, `Manager`, `Editor`, `Approver`, `Viewer`, `Client`.
- Timezone and currency settings per brand.
- Audit trail logging all brand updates and member invitations.

### 2. Multi-Platform Planner & Smart Scheduler
- **Cross-Platform Scheduling**: Instagram, Facebook, LinkedIn, Twitter/X, TikTok, YouTube, Pinterest, Google Business Profile.
- **Post Formats**: Single images, Videos, Reels, Stories, Shorts, Multi-image/video Carousels, and PDF Document carousels.
- **Platform-Specific Options**:
  - Auto-posted first comment (hashtags).
  - User tags and collaborators.
  - Reels audio/cover.
  - Pinterest boards and destination links.
- **Scheduling Modes**:
  - Instant publish (`now`).
  - One-time scheduled (`scheduled`).
  - **Autolists / Evergreen Queues**: Recycle evergreen content on predefined weekly day/time slots with circular repetition or max repetitions.
  - **RSS Feed Auto-Scheduler**: Automatically sync and schedule posts from external RSS blogs/news feeds using customizable caption templates.
- **Approval Workflow**:
  - Multi-tier review: `draft` -> `pending_approval` -> `approved` -> `scheduled` -> `published` / `rejected`.
  - Public Client Review links (`/review/:token`) allowing external clients to review, approve, or reject drafts with comments without logging in.

### 3. Best Times to Post (7x24 Heatmaps)
- Algorithmic 7-day x 24-hour heatmap calculation per social network (Instagram, Facebook, TikTok, LinkedIn, YouTube, Twitter/X).
- Computes peak audience active windows and engagement probability scores (0.0 to 100.0).

### 4. Unified Social Inbox & Community Management
- Centralized inbox consolidating DMs, comments, and reviews across Instagram, Facebook, LinkedIn, TikTok, Twitter/X, and Google Business Profile.
- Real-time conversation threads, read/unread status, and open/resolved state tracking.
- **Saved / Canned Replies** with quick shortcut triggers (e.g. `/thanks`).
- Automated sentiment analysis (`positive`, `neutral`, `negative`) and conversation tagging.
- Team member assignment.

### 5. SmartLinks (Link-in-Bio Platform)
- Customizable Link-in-Bio landing pages hosted at `/bio/:slug`.
- Custom styling: theme presets (`light`, `dark`, `gradient`, `minimal`), custom avatar, bio, and social icon links.
- Diverse component blocks:
  - Classic link buttons with scheduled activation dates.
  - Image banners and media embeds.
  - Product cards and catalog items.
  - Social feed grid (links mapped directly to Instagram/TikTok posts).
- Real-time click and view analytics with CTR calculations.

### 6. Competitor Benchmarking & Tracking
- Benchmark competitors across Instagram, Facebook, YouTube, Twitch, and Twitter/X.
- Track follower growth, post frequency, and engagement rates over time with historical snapshots.

### 7. Paid Ad Campaigns Tracker
- Unified ad dashboard across **Google Ads**, **Meta (Facebook/Instagram) Ads**, and **TikTok Ads**.
- Aggregate and campaign-level metrics: Budget, Spend, Impressions, Clicks, Conversions, CTR, CPC, CPM, and ROAS.

### 8. Web & Social Analytics
- **Web Analytics**:
  - 1x1 GIF tracking pixel (`GET /tracker/pixel.png`) and JavaScript beacon (`POST /tracker/event`) for tracking website traffic.
  - Metrics: Pageviews, unique visitors, referrers, countries, and devices.
- **Social Network Analytics**:
  - Historical rollups by day, week, and month for follower growth, reach, impressions, engagement, video views, and clicks.

### 9. Custom & White-Label Reporting
- Generate branded performance reports in **PDF**, **CSV**, or **JSON** format.
- White-label customization: custom agency logo and brand colors.
- Automated report archiving and direct client downloads.

### 10. Webhooks & Real-Time Events
- Webhook registration for events (`post.published`, `post.failed`, `inbox.message`, `review.action`, `report.ready`).
- HMAC-SHA256 signature verification headers (`X-XSocial-Signature`).

---

## API Key Access (Developer REST API)

Developers and automated services can access all API endpoints using scoped API keys.

### Authentication Header
```http
Authorization: Bearer mc_live_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### Permissions / Scopes
- `full_access`: Unrestricted access across all brand resources.
- `analytics:read`: Read-only access to web and social analytics.
- `planner:write`: Create, edit, and schedule posts.
- `inbox:manage`: Read and reply to inbox conversations.
- `smartlinks:manage`: Manage link-in-bio landing pages.
- `ads:read`: Access paid advertising metrics.
- `reports:read`: Generate and download reports.

### API Key Endpoints
- `POST /api/v1/api-keys`: Create a new API key with custom name, permissions, and brand scoping.
- `GET /api/v1/api-keys`: List all active API keys (showing prefix, last four digits, and last used time).
- `DELETE /api/v1/api-keys/:id`: Revoke an API key immediately.

---

## Model Context Protocol (MCP) Access

The backend provides a built-in remote Model Context Protocol (MCP) server adhering to protocol specification `2024-11-05` over HTTP JSON-RPC 2.0 at:

```
POST /mcp
GET  /mcp
```
(also accessible at `/api/v1/mcp`)

Connect AI clients (such as Claude Desktop, Cursor, typingmind, Antigravity) by providing the endpoint URL and your API Key.

### Available MCP Tools

| Tool Name | Description |
| :--- | :--- |
| `get_brands` | List all managed brands and client workspaces. |
| `get_brand_details` | Retrieve settings, timezone, and accounts for a brand. |
| `create_brand` | Create a new brand/client workspace. |
| `get_analytics` | Fetch performance metrics across channels (Instagram, Facebook, LinkedIn, TikTok, YouTube, Web). |
| `get_best_time_to_post` | Get the 7x24 heatmap and recommended hours to post. |
| `get_competitors` | Retrieve competitor benchmark metrics. |
| `get_scheduled_posts` | List scheduled, draft, or published posts. |
| `create_scheduled_post` | Create and schedule a post with media, platforms, and first comment. |
| `publish_post_now` | Immediately publish a post across selected platforms. |
| `approve_post` | Approve a post in the review approval workflow. |
| `list_inbox_messages` | Fetch unified inbox conversation threads (DMs, comments, reviews). |
| `reply_inbox_message` | Send a reply to an inbox conversation. |
| `get_smartlinks` | Retrieve SmartLink (link-in-bio) pages and analytics. |
| `get_ad_campaigns` | Query Google Ads, Meta Ads, and TikTok Ads metrics. |
| `generate_report` | Generate a performance report (PDF, CSV, or JSON). |

---

## Getting Started

### Prerequisites
- **Go 1.24+**
- **MongoDB** (v6.0+)
- **Redis** (v7.0+)

### Setup

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Build the binary:
   ```bash
   make build
   ```

3. Run the server:
   ```bash
   make run
   ```

The server will start on port `8000` (or the configured `PORT`), listening for HTTP requests, MCP JSON-RPC calls, and processing background scheduler queues.
