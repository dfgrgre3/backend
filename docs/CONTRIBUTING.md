# Contributing to Thanawy Backend

Thank you for your interest in contributing to the Thanawy Backend project!

## Development Setup

### Prerequisites
- Go 1.21 or higher
- PostgreSQL 14 or higher
- Redis 6 or higher
- Node.js 18+ (for dev tooling)

### Getting Started

1. Clone the repository:
```bash
git clone <repository-url>
cd backend
```

2. Copy environment variables:
```bash
cp .env.example .env
```

3. Configure your `.env` file with your database and Redis credentials.

4. Install Go dependencies:
```bash
go mod download
```

5. Run database migrations:
```bash
go run cmd/migrate/main.go
```

6. Start the development server:
```bash
go run cmd/api/main.go
```

## Project Structure

```
backend/
├── cmd/                    # Entry points for different applications
│   ├── api/               # Main API server
│   └── migrate/           # Database migration tool
├── internal/
│   ├── api/               # HTTP handlers and gRPC services
│   ├── app/               # Application initialization and DI
│   ├── bootstrap/         # Application bootstrap logic
│   ├── config/            # Configuration management
│   ├── cqrs/              # CQRS pattern implementation
│   ├── db/                # Database connection and migrations
│   ├── middleware/        # HTTP middleware
│   ├── models/            # Domain models
│   ├── repository/        # Data access layer
│   ├── router/            # Route definitions
│   ├── services/          # Business logic services
│   └── worker/            # Background workers
├── pkg/                   # Public packages
├── scripts/               # Utility scripts
└── tools/                 # Development tools
```

## Code Style

We follow standard Go conventions:
- Use `gofmt` for formatting
- Follow effective Go guidelines
- Write clear, self-documenting code
- Add comments for exported functions and complex logic

## Testing

Run tests with:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

## Database Migrations

Create a new migration:
```bash
go run cmd/migrate/main.go create <migration_name>
```

Run migrations:
```bash
go run cmd/migrate/main.go up
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Ensure all tests pass
6. Submit a pull request with a clear description

## Code Review Guidelines

- Keep PRs focused and small
- Update documentation as needed
- Follow the existing code style
- Ensure no breaking changes without discussion
- Add tests for new features

## Questions?

Feel free to open an issue for questions or discussions.
