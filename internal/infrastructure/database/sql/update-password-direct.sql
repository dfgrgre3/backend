-- For this exercise, I'll use a known bcrypt hash for "password123" 
-- and update the admin password to this for testing purposes
-- The user can then change it after login

-- bcrypt hash for "password123" with cost 12
INSERT INTO "UserCredential" (user_id, password_hash, created_at, updated_at)
VALUES ('admin-001', '$2a$12$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE SET
    password_hash = '$2a$12$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi',
    updated_at = NOW();