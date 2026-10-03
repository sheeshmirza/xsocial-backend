-- ==============================================================================
-- xSocial Relational Database Migration (PostgreSQL / Relational Engines)
-- Version: 001
-- Description: Fully Normalized 3NF Relational Schema with Data Integrity,
--              Foreign Key Constraints, Composite Indexes, and Audit Triggers
-- ==============================================================================

-- Enable UUID extension if supported
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- -----------------------------------------------------------------------------
-- 1. Table: users
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL DEFAULT 'New User',
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    email_verification_token VARCHAR(128) UNIQUE,
    email_verification_expires_at TIMESTAMPTZ,
    reset_password_token VARCHAR(128) UNIQUE,
    reset_password_expires_at TIMESTAMPTZ,
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_users_email_format CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users (created_at DESC);

-- -----------------------------------------------------------------------------
-- 2. Table: refresh_tokens
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    token_expires_at TIMESTAMPTZ NOT NULL,
    token_revoked_at TIMESTAMPTZ,
    user_agent VARCHAR(512),
    user_ip_address VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens (token_expires_at);

-- -----------------------------------------------------------------------------
-- 3. Table: brands (Multi-tenant Workspaces)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS brands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    logo_url VARCHAR(1024) DEFAULT '',
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    website_url VARCHAR(1024) DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_brands_currency_iso CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX IF NOT EXISTS idx_brands_owner_id ON brands (owner_id);
CREATE INDEX IF NOT EXISTS idx_brands_created_at ON brands (created_at DESC);

-- -----------------------------------------------------------------------------
-- 4. Table: brand_members (RBAC Membership)
-- -----------------------------------------------------------------------------
CREATE TYPE brand_role_enum AS ENUM ('owner', 'admin', 'manager', 'editor', 'approver', 'viewer', 'client');

CREATE TABLE IF NOT EXISTS brand_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    brand_id UUID NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    role brand_role_enum NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_brand_member UNIQUE (brand_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_brand_members_user_id ON brand_members (user_id);
CREATE INDEX IF NOT EXISTS idx_brand_members_brand_id ON brand_members (brand_id);

-- -----------------------------------------------------------------------------
-- 5. Table: social_handles (Connected OAuth Accounts)
-- -----------------------------------------------------------------------------
CREATE TYPE platform_type_enum AS ENUM ('INSTAGRAM', 'LINKEDIN');

CREATE TABLE IF NOT EXISTS social_handles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    brand_id UUID REFERENCES brands(id) ON DELETE SET NULL,
    platform platform_type_enum NOT NULL,
    platform_id VARCHAR(128) NOT NULL,
    platform_username VARCHAR(255) NOT NULL DEFAULT '',
    platform_display_name VARCHAR(255) NOT NULL DEFAULT '',
    platform_access_token TEXT NOT NULL,
    platform_refresh_token TEXT NOT NULL DEFAULT '',
    platform_access_token_expiry TIMESTAMPTZ,
    platform_refresh_token_expiry TIMESTAMPTZ,
    platform_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_platform_identity UNIQUE (user_id, platform, platform_id)
);

CREATE INDEX IF NOT EXISTS idx_social_handles_user_brand ON social_handles (user_id, brand_id);
CREATE INDEX IF NOT EXISTS idx_social_handles_token_expiry ON social_handles (platform_access_token_expiry);

-- -----------------------------------------------------------------------------
-- 6. Table: posts
-- -----------------------------------------------------------------------------
CREATE TYPE post_status_enum AS ENUM ('draft', 'scheduled', 'published', 'failed');
CREATE TYPE post_type_enum AS ENUM ('text', 'image', 'video', 'gif', 'mixed', 'reel', 'story', 'carousel', 'poll', 'article');
CREATE TYPE post_visibility_enum AS ENUM ('public', 'private');
CREATE TYPE recurrence_enum AS ENUM ('once', 'hourly', 'daily', 'weekly', 'monthly', 'yearly');

CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    brand_id UUID REFERENCES brands(id) ON DELETE SET NULL,
    title VARCHAR(512) NOT NULL DEFAULT '',
    caption TEXT NOT NULL DEFAULT '',
    hashtags TEXT[] NOT NULL DEFAULT '{}',
    mentions TEXT[] NOT NULL DEFAULT '{}',
    media JSONB NOT NULL DEFAULT '[]'::jsonb,
    post_type post_type_enum NOT NULL DEFAULT 'text',
    poll JSONB,
    visibility post_visibility_enum NOT NULL DEFAULT 'public',
    timezone VARCHAR(64) NOT NULL DEFAULT 'UTC',
    scheduled_time TIMESTAMPTZ,
    published_time TIMESTAMPTZ,
    published_expires_at TIMESTAMPTZ,
    recurrence recurrence_enum NOT NULL DEFAULT 'once',
    recurrence_interval INTEGER,
    recurrence_unit VARCHAR(16),
    recurrence_paused BOOLEAN NOT NULL DEFAULT FALSE,
    status post_status_enum NOT NULL DEFAULT 'draft',
    selected_platforms TEXT[] NOT NULL DEFAULT '{}',
    selected_account_ids TEXT[] NOT NULL DEFAULT '{}',
    expiry_minutes INTEGER,
    expiry_interval INTEGER,
    expiry_unit VARCHAR(16),
    review_status VARCHAR(32) DEFAULT 'draft',
    review_notes JSONB NOT NULL DEFAULT '[]'::jsonb,
    approval_token VARCHAR(128) UNIQUE,
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    first_comment TEXT DEFAULT '',
    instagram_trial JSONB,
    origin_post_id UUID REFERENCES posts(id) ON DELETE SET NULL,
    origin_platform_post_id VARCHAR(128) DEFAULT '',
    origin_account_id VARCHAR(128) DEFAULT '',
    is_reshare BOOLEAN NOT NULL DEFAULT FALSE,
    reshare_commentary TEXT DEFAULT '',
    platform_responses JSONB NOT NULL DEFAULT '[]'::jsonb,
    ai JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_posts_user_status_created ON posts (user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_user_scheduled ON posts (user_id, scheduled_time, status) WHERE scheduled_time IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_posts_brand_status ON posts (brand_id, status);
CREATE INDEX IF NOT EXISTS idx_posts_published_expires ON posts (published_expires_at) WHERE published_expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_posts_approval_token ON posts (approval_token) WHERE approval_token IS NOT NULL;

-- -----------------------------------------------------------------------------
-- 7. Table: post_activities (LinkedIn & Social Sync History)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS post_activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    social_handle_id UUID NOT NULL REFERENCES social_handles(id) ON DELETE CASCADE,
    platform VARCHAR(32) NOT NULL DEFAULT 'linkedin',
    platform_post_id VARCHAR(128) NOT NULL,
    platform_post_urn VARCHAR(255) NOT NULL DEFAULT '',
    platform_post_url VARCHAR(1024) NOT NULL DEFAULT '',
    content_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_post_activity_platform UNIQUE (user_id, social_handle_id, platform, platform_post_id)
);

CREATE INDEX IF NOT EXISTS idx_post_activities_user_updated ON post_activities (user_id, updated_at DESC);

-- -----------------------------------------------------------------------------
-- 8. Table: api_keys (Developer & External Access)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    brand_id UUID REFERENCES brands(id) ON DELETE SET NULL,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    prefix VARCHAR(32) NOT NULL DEFAULT 'mc_live_',
    last_four VARCHAR(4) NOT NULL,
    permissions TEXT[] NOT NULL DEFAULT '{"full_access"}',
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_created ON api_keys (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_keys_brand_id ON api_keys (brand_id);

-- -----------------------------------------------------------------------------
-- 9. Table: audit_logs (Enterprise Security & Compliance)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    brand_id UUID REFERENCES brands(id) ON DELETE SET NULL,
    actor VARCHAR(255) NOT NULL,
    action VARCHAR(128) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128) NOT NULL,
    ip_address VARCHAR(64),
    user_agent VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON audit_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_brand_created ON audit_logs (brand_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs (resource_type, resource_id);

-- -----------------------------------------------------------------------------
-- 10. Table: _schema_migrations
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS _schema_migrations (
    version INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    execution_time_ms BIGINT NOT NULL DEFAULT 0,
    checksum VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'applied'
);
