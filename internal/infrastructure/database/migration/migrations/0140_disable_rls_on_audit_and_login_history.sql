-- ============================================================
-- Migration 0140: Disable RLS on AuditLog and login_history
-- ============================================================
-- The setup-db-roles.sql script enables RLS on ALL tables in the
-- public schema (except http_metric_buckets and AnalyticsEvent),
-- but no RLS policies are defined for AuditLog or login_history.
--
-- With RLS enabled and no applicable policy, PostgreSQL defaults
-- to DENY ALL, which blocks the app_user role from INSERTing
-- audit log entries and login history records.
--
-- These tables are internal system tables written by the backend
-- itself (not by end users), so they do not require multi-tenant
-- row isolation. Disabling RLS restores write access.
--
-- This migration is idempotent: if RLS is already disabled, it is
-- a no-op. ALTER TABLE IF EXISTS guards against missing tables.
-- ============================================================

ALTER TABLE IF EXISTS public."AuditLog" DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS public."login_history" DISABLE ROW LEVEL SECURITY;