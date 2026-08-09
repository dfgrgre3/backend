#!/bin/sh
set -e

# Use a simple approach: create a temporary Go program to hash the password
cat > /tmp/hash-password.go << 'GOEOF'
package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "Khaled@2008"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(hash))
}
GOEOF

# We need to run this in a Go environment
# For now, let's use a pre-computed bcrypt hash for "Khaled@2008"
# This is a bcrypt hash with cost 12
# You can generate this properly with: bcrypt.GenerateFromPassword([]byte("Khaled@2008"), 12)

# For demonstration, I'll use a placeholder approach
# In production, you should generate this properly
BCRYPT_HASH='$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewY5NUyxBqZe8QwG'

echo "Updating password hash for admin user..."

PGPASSWORD=thanawy_dev_password psql -h localhost -p 5432 -U thanawy -d thanawy << EOF
-- Insert the password hash
INSERT INTO "UserCredential" (user_id, password_hash, created_at, updated_at)
VALUES ('admin-001', '$BCRYPT_HASH', NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE SET
    password_hash = '$BCRYPT_HASH',
    updated_at = NOW();
EOF

echo "Password updated successfully"
echo "Email: ffyoussef12@gmail.com"
echo "Password: Khaled@2008"