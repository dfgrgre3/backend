# Migrations

Applied in `sort.Strings()` order over the full filename (see `../migration_apply.go`
and `../migration.go`), and tracked in `schema_migrations` keyed by filename
(without the `.sql` extension). This means:

- Ordering is **deterministic and identical across every environment** — it
  does not depend on numeric parsing, only on byte-wise string sort of the
  filename.
- **Never rename an already-applied migration file.** Renaming changes its
  tracked id, so the migrator will treat it as a new pending migration and
  try to re-apply it (fails, or silently diverges if the SQL is `IF NOT
  EXISTS`-guarded). If a filename must change, do it only for a migration
  that has not yet been applied anywhere.

## Known duplicate number prefixes

A handful of migrations share the same numeric prefix (added independently by
different branches without checking for collisions). All are already applied
in existing environments, so they are left as-is rather than renumbered —
renumbering would trigger re-application (see above). Actual applied order
for each pair, per alphabetical filename sort:

| Prefix | Applied first | Applied second |
|---|---|---|
| 0055 | `0055_add_certificates.sql` | `0055_add_super_admin_role.sql` |
| 0112 | `0112_add_affiliates_advanced.sql` | `0112_lms_core_tables.sql` |
| 0115 | `0115_create_credentials_tables.sql` | `0115_make_refresh_token_nullable.sql` |
| 0116 | `0116_create_mail_tasks.sql` | `0116_make_refresh_token_column_non_null_default.sql` |
| 0161 | `0161_add_user_admin_note.sql` | `0161_announcements_is_active.sql` |

There is also a numbering gap (0065–0107 do not exist) from earlier history
squashing/renumbering — harmless, the migrator does not require contiguous
numbers.

**New migrations:** pick a number higher than the current max regardless of
these gaps/collisions, and make it unique. Two new migrations sharing a
prefix is not a functional bug (order is still deterministic) but makes
intent harder to read — avoid it when you notice a collision at PR time.
