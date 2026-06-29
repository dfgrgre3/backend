-- Migration to add browser, os, and country to UserSession
ALTER TABLE IF EXISTS public."UserSession"
    ADD COLUMN IF NOT EXISTS browser text,
    ADD COLUMN IF NOT EXISTS os text,
    ADD COLUMN IF NOT EXISTS country text;
