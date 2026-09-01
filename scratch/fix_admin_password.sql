-- Update password for admin user ffyoussef12@gmail.com
-- bcrypt hash for "Khaled@2008" with cost 12
-- Generated: 2026-08-29
UPDATE "UserCredential" 
SET 
    password_hash = '$2a$12$T0STDRQhbeY0oUsOtgaQbeweaJJj4nY3siFDdOuGUYcfGPYjGUwGG',
    updated_at = NOW()
WHERE user_id = '19291e50-af25-493d-9cf6-89a41d6a0abf';

-- Verify the update
SELECT uc.user_id, substring(uc.password_hash, 1, 30) as hash_preview, u.email, u.role, u.status
FROM "UserCredential" uc
JOIN "User" u ON u.id = uc.user_id
WHERE uc.user_id = '19291e50-af25-493d-9cf6-89a41d6a0abf';
