# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- .env.example file with comprehensive environment variable documentation
- docs/architecture.md with detailed architecture documentation
- Moved test tools from cmd/ to tools/ for better organization

### Changed
- Updated .gitignore to allow docs/*.md files
- Clarified database architecture: GORM is primary system, Prisma removed

### Removed
- prisma.config.ts (Prisma CLI tool no longer needed)
- cmd/check_db_constraints/ (moved to tools/)
- cmd/test_cascade/ (moved to tools/)
- cmd/test_ai/ (empty directory removed)
- cmd/fix_login_alerts/ (empty directory removed)
