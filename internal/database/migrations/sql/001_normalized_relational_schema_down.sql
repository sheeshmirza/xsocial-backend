-- ==============================================================================
-- xSocial Relational Database Rollback (PostgreSQL)
-- Version: 001
-- ==============================================================================

DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS post_activities CASCADE;
DROP TABLE IF EXISTS posts CASCADE;
DROP TABLE IF EXISTS social_handles CASCADE;
DROP TABLE IF EXISTS brand_members CASCADE;
DROP TABLE IF EXISTS brands CASCADE;
DROP TABLE IF EXISTS refresh_tokens CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS _schema_migrations CASCADE;

DROP TYPE IF EXISTS recurrence_enum CASCADE;
DROP TYPE IF EXISTS post_visibility_enum CASCADE;
DROP TYPE IF EXISTS post_type_enum CASCADE;
DROP TYPE IF EXISTS post_status_enum CASCADE;
DROP TYPE IF EXISTS platform_type_enum CASCADE;
DROP TYPE IF EXISTS brand_role_enum CASCADE;
