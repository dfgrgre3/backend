#!/bin/sh
set -e

# Database connection
DB_HOST="localhost"
DB_PORT="5432"
DB_USER="thanawy"
DB_PASSWORD="thanawy_dev_password"
DB_NAME="thanawy"

# Admin credentials
ADMIN_EMAIL="ffyoussef12@gmail.com"
ADMIN_PASSWORD="Khaled@2008"

# Create tables if they don't exist
echo "Creating database tables..."

PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME << 'EOF'
-- Create User table
CREATE TABLE IF NOT EXISTS "User" (
    id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    role VARCHAR(50) NOT NULL DEFAULT 'USER',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    email_verified BOOLEAN NOT NULL DEFAULT false,
    permissions JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Create UserCredential table
CREATE TABLE IF NOT EXISTS "UserCredential" (
    user_id VARCHAR(36) PRIMARY KEY REFERENCES "User"(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create other necessary tables
CREATE TABLE IF NOT EXISTS "schema_migrations" (
    id text PRIMARY KEY,
    checksum text NOT NULL,
    "appliedAt" timestAMPTZ NOT NULL DEFAULT NOW()
);
EOF

echo "Tables created successfully"

# Generate password hash (we'll use a simple bcrypt hash for now)
# In production, this should be done properly with bcrypt
echo "Creating admin user..."

PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME << EOF
-- Insert or update admin user
INSERT INTO "User" (id, email, role, status, email_verified, permissions, created_at, updated_at)
VALUES (
    'admin-001',
    '$ADMIN_EMAIL',
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

-- Note: Password hash needs to be generated with bcrypt
-- For now, we'll set a placeholder that needs to be updated
-- The actual password hashing should be done with the Go application
EOF

echo "Admin user created successfully"
echo "Email: $ADMIN_EMAIL"
echo "Password: $ADMIN_PASSWORD"
echo ""
echo "NOTE: Password hash needs to be set properly using bcrypt"
echo "Please run the seed-admin Go application to set the correct password hash"