-- Migration: 0055_add_super_admin_role.sql
-- Description: Add SUPER_ADMIN to the UserRole enum in PostgreSQL

ALTER TYPE public."UserRole" ADD VALUE 'SUPER_ADMIN';
