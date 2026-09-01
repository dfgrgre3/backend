-- bcrypt hash for "Khaled@2008" with cost 12
-- Generated: 2026-08-29
INSERT INTO "UserCredential" (user_id, password_hash, created_at, updated_at)
VALUES ('admin-001', '$2a$12$T0STDRQhbeY0oUsOtgaQbeweaJJj4nY3siFDdOuGUYcfGPYjGUwGG', NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE SET
    password_hash = '$2a$12$T0STDRQhbeY0oUsOtgaQbeweaJJj4nY3siFDdOuGUYcfGPYjGUwGG',
    updated_at = NOW();