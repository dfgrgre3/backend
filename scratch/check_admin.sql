-- Check user table structure and admin user
SELECT id, email, role, status, email_verified FROM "User" WHERE email = 'ffyoussef12@gmail.com';

-- Check if there are any admin users
SELECT id, email, role, status FROM "User" WHERE role IN ('SUPER_ADMIN', 'ADMIN') LIMIT 5;

-- Check UserCredential table
SELECT uc.user_id, substring(uc.password_hash, 1, 30) as hash_preview, u.email 
FROM "UserCredential" uc 
JOIN "User" u ON u.id = uc.user_id
WHERE u.role IN ('SUPER_ADMIN', 'ADMIN')
LIMIT 5;
