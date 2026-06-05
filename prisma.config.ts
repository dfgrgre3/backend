// ============================================================
// Prisma Configuration — Thanawy Backend
// ============================================================
// Prisma is used ONLY as a migration/introspection CLI tool here.
// All runtime database access uses GORM (Go ORM) via the
// internal/db package.
//
// Commands:
//   npx prisma migrate deploy        — apply pending migrations to prod DB
//   npx prisma db pull               — introspect live DB → regenerate schema
//   npx prisma migrate dev --name X  — create a new migration file
//   npx prisma studio                — open DB GUI
//
// Requires .env (or exported env vars) with:
//   DATABASE_URL  = postgresql://...@pooler.supabase.com:6543/postgres?pgbouncer=true
//   DIRECT_URL    = postgresql://...@db.supabase.co:5432/postgres
// ============================================================

import "dotenv/config";
import { defineConfig } from "prisma/config";

export default defineConfig({
  schema: "prisma/schema.prisma",
  migrations: {
    path: "prisma/migrations",
  },
  datasource: {
    // Pooled URL (PgBouncer, port 6543) — for all runtime queries
    url: process.env["DATABASE_URL"],
    // Direct URL (port 5432) — required for Prisma Migrate (DDL statements
    // cannot run through PgBouncer transaction-mode pooling)
    directUrl: process.env["DIRECT_URL"],
  },
});
