-- Migration: 0169_seed_system_roles_and_assignments.sql
-- Purpose  : Seed the canonical system roles and wire them to their default
--            permission sets so the admin /admin/roles page and the runtime
--            mergeCustomRolePermissions() path both have a meaningful starting
--            point. Idempotent: safe to re-run.
-- Notes    :
--   * The runtime auth path still uses User.role (enum) + User.permissions as
--     the source of truth, and this migration only populates the custom
--     management tables (roles / role_permissions). The merge is additive on
--     top of the enum-role defaults (see mergeCustomRolePermissions).
--   * "level" follows a 0..100 hierarchy: higher = more privileged. It is
--     used by the admin UI to order/sort roles and to gate role creation.
--   * This migration assumes 0168_permissions_action_and_catalog.sql has
--     already populated the permissions table; we only reference existing
--     permission names.

BEGIN;

-- 1) Seed system roles (level drives UI sort + privilege hierarchy)
--    Use ON CONFLICT DO NOTHING to avoid colliding with pre-existing rows
--    (e.g. legacy PARENT/SUPPORT seeded by 0063). We then UPDATE their
--    description/level/UUID in a separate idempotent statement so the
--    canonical UUIDs above are preserved on first run.
INSERT INTO roles (id, name, description, is_system, level) VALUES
  ('00000000-0000-0000-0000-0000000000a1', 'SUPER_ADMIN', 'Full system access. Bypasses all permission checks.',       true, 100),
  ('00000000-0000-0000-0000-0000000000a2', 'ADMIN',       'Operational administrator (no admin:bypass).',                true,  80),
  ('00000000-0000-0000-0000-0000000000a3', 'MODERATOR',   'Content moderation + support queue visibility.',              true,  60),
  ('00000000-0000-0000-0000-0000000000a4', 'SUPPORT',     'Customer support agent: tickets, FAQs, comments.',           true,  50),
  ('00000000-0000-0000-0000-0000000000a5', 'TEACHER',     'Teacher: own content + student view.',                       true,  40),
  ('00000000-0000-0000-0000-0000000000a6', 'PARENT',      'Parent: own children + progress.',                           true,  20),
  ('00000000-0000-0000-0000-0000000000a7', 'STUDENT',     'Student: consume content, take exams, participate.',         true,  10)
ON CONFLICT (name) DO NOTHING;

-- Realign description/is_system/level on existing rows so legacy seeds
-- (PARENT/SUPPORT from migration 0063) get the same canonical metadata.
UPDATE roles SET
  description = CASE name
    WHEN 'SUPER_ADMIN' THEN 'Full system access. Bypasses all permission checks.'
    WHEN 'ADMIN'       THEN 'Operational administrator (no admin:bypass).'
    WHEN 'MODERATOR'   THEN 'Content moderation + support queue visibility.'
    WHEN 'SUPPORT'     THEN 'Customer support agent: tickets, FAQs, comments.'
    WHEN 'TEACHER'     THEN 'Teacher: own content + student view.'
    WHEN 'PARENT'      THEN 'Parent: own children + progress.'
    WHEN 'STUDENT'     THEN 'Student: consume content, take exams, participate.'
    ELSE description
  END,
  is_system = CASE
    WHEN name IN ('SUPER_ADMIN','ADMIN','MODERATOR','SUPPORT','TEACHER','PARENT','STUDENT') THEN true
    ELSE is_system
  END,
  level = CASE name
    WHEN 'SUPER_ADMIN' THEN 100
    WHEN 'ADMIN'       THEN  80
    WHEN 'MODERATOR'   THEN  60
    WHEN 'SUPPORT'     THEN  50
    WHEN 'TEACHER'     THEN  40
    WHEN 'PARENT'      THEN  20
    WHEN 'STUDENT'     THEN  10
    ELSE level
  END
WHERE name IN ('SUPER_ADMIN','ADMIN','MODERATOR','SUPPORT','TEACHER','PARENT','STUDENT');

-- 2) Insert role_permissions for each seeded role. We do this OUTSIDE a
--    DO block so that an error in one role does not roll back the whole
--    transaction (the prior version did, leaving orphaned roles with no
--    permissions). All inserts use ON CONFLICT DO NOTHING to be idempotent.

-- 2.1) SUPER_ADMIN gets every permission currently in the catalog
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'SUPER_ADMIN'
ON CONFLICT DO NOTHING;

-- 2.2) ADMIN
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'ADMIN' AND p.name IN (
  'dashboard:view','analytics:view','reports:view','reports:manage','audit_logs:view',
  'users:view','users:create','users:update','users:delete','users:manage','users:impersonate','users:export','users:import',
  'students:view','students:manage',
  'teachers:view','teachers:manage',
  'parents:view','parents:manage',
  'subjects:view','subjects:create','subjects:update','subjects:delete','subjects:manage','subjects:publish','subjects:approve',
  'books:view','books:create','books:update','books:delete','books:manage','books:publish',
  'resources:view','resources:manage','resources:publish',
  'exams:view','exams:create','exams:update','exams:delete','exams:manage','exams:approve','exams:publish',
  'challenges:view','challenges:manage',
  'contests:view','contests:manage',
  'blog:view','blog:create','blog:update','blog:delete','blog:manage','blog:publish',
  'forum:view','forum:create','forum:update','forum:delete','forum:moderate','forum:manage',
  'comments:view','comments:create','comments:moderate',
  'events:view','events:manage',
  'announcements:view','announcements:manage',
  'tickets:view','tickets:create','tickets:update','tickets:manage','tickets:resolve','faqs:manage',
  'achievements:view','achievements:manage',
  'rewards:view','rewards:manage',
  'ai:manage','ai:usage',
  'live_monitor:view','marketing:view','marketing:manage',
  'ab_testing:view','settings:view',
  'seasons:view','seasons:manage',
  'system:manage','system:settings',
  'notifications:send','notifications:manage'
)
ON CONFLICT DO NOTHING;

-- 2.3) MODERATOR
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'MODERATOR' AND p.name IN (
  'dashboard:view','analytics:view','reports:view',
  'dashboard:access','dashboard:view_kpis',
  'dashboard:view_learning_metrics','dashboard:view_content_metrics',
  'dashboard:view_support_metrics','dashboard:view_recent_activity',
  'dashboard:view_pending_items','dashboard:view_alerts',
  'dashboard:view_top_courses','dashboard:acknowledge_alerts',
  'dashboard:save_filters','dashboard:delete_saved_filters','dashboard:apply_saved_filters',
  'users:view','students:view','teachers:view','parents:view',
  'subjects:view','subjects:approve',
  'books:view','books:publish',
  'resources:view','resources:publish',
  'exams:view','exams:approve','exams:publish',
  'challenges:view','contests:view',
  'blog:view','blog:create','blog:update','blog:delete','blog:publish',
  'forum:view','forum:create','forum:update','forum:delete','forum:moderate',
  'comments:view','comments:moderate',
  'events:view','events:manage',
  'announcements:view','announcements:manage',
  'tickets:view','tickets:manage','tickets:resolve',
  'achievements:view','rewards:view',
  'live_monitor:view','marketing:view','settings:view',
  'notifications:send','notifications:manage'
)
ON CONFLICT DO NOTHING;

-- 2.4) SUPPORT
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'SUPPORT' AND p.name IN (
  'dashboard:view',
  'dashboard:access','dashboard:view_support_metrics',
  'dashboard:view_pending_items','dashboard:view_alerts',
  'dashboard:acknowledge_alerts','dashboard:apply_saved_filters',
  'users:view','students:view','teachers:view','parents:view',
  'tickets:view','tickets:create','tickets:update','tickets:manage','tickets:resolve',
  'faqs:manage',
  'forum:view','comments:view',
  'announcements:view','notifications:send','settings:view'
)
ON CONFLICT DO NOTHING;

-- 2.5) TEACHER
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'TEACHER' AND p.name IN (
  'dashboard:view','analytics:view',
  'students:view',
  'subjects:view','own_subjects:manage',
  'books:view','own_books:manage',
  'resources:view','own_resources:manage',
  'exams:view','own_exams:manage',
  'challenges:view','own_challenges:manage',
  'blog:view','blog:create','blog:update',
  'forum:view','forum:create',
  'comments:view','comments:create',
  'achievements:view','rewards:view','ai:usage'
)
ON CONFLICT DO NOTHING;

-- 2.6) PARENT
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'PARENT' AND p.name IN (
  'dashboard:view',
  'children:view','children:grades','children:progress','children:attendance','children:communicate','children:payment',
  'subjects:view','books:view','exams:view',
  'blog:view','forum:view','comments:view',
  'achievements:view','notifications:send'
)
ON CONFLICT DO NOTHING;

-- 2.7) STUDENT
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'STUDENT' AND p.name IN (
  'dashboard:view','analytics:view',
  'subjects:view','books:view','resources:view','exams:view','challenges:view',
  'blog:view','forum:view','forum:create',
  'comments:view','comments:create',
  'achievements:view','rewards:view','ai:usage'
)
ON CONFLICT DO NOTHING;

-- 3) Auto-assign SUPER_ADMIN role to existing users with role='SUPER_ADMIN'
--    so the admin UI shows them in the user_roles table immediately.
INSERT INTO user_roles (user_id, role_id, assigned_at)
SELECT u.id, r.id, now()
FROM "User" u
JOIN roles r ON r.name = 'SUPER_ADMIN'
WHERE u.role = 'SUPER_ADMIN'
  AND NOT EXISTS (
    SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id AND ur.role_id = r.id
  );

-- 4) Performance indexes for the admin RBAC queries.
CREATE INDEX IF NOT EXISTS idx_role_permissions_role          ON role_permissions (role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_user                ON user_roles (user_id);
CREATE INDEX IF NOT EXISTS idx_user_permissions_user          ON user_permissions (user_id);
CREATE INDEX IF NOT EXISTS idx_roles_level                    ON roles (level);

COMMIT;
