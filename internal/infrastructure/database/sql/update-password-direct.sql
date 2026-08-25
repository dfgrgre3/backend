-- Update the admin password using a parameterized hash
-- The caller must supply the bcrypt hash as :password_hash

INSERT INTO "UserCredential" (user_id, password_hash, created_at, updated_at)
VALUES ('admin-001', :password_hash, NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE SET
    password_hash = :password_hash,
    updated_at = NOW();