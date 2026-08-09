-- Insert admin user
INSERT INTO "User" (id, email, role, status, email_verified, permissions, created_at, updated_at)
VALUES (
    'admin-001',
    'ffyoussef12@gmail.com',
    'SUPER_ADMIN',
    'ACTIVE',
    true,
    '["admin:all", "user:read", "user:write", "content:read", "content:write", "system:manage"]'::jsonb,
    NOW(),
    NOW()
)
ON CONFLICT (email) DO UPDATE SET
    role = 'SUPER_ADMIN',
    status = 'ACTIVE',
    email_verified = true,
    permissions = '["admin:all", "user:read", "user:write", "content:read", "content:write", "system:manage"]'::jsonb,
    updated_at = NOW();

-- Note: We need to insert the password hash separately
-- The password hash for "Khaled@2008" needs to be generated with bcrypt
-- For now, we'll insert a placeholder and the Go application should update it