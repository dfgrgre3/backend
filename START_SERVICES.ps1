# Thanawy Backend - Start All Services
# This script starts all required services using Docker Compose

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "  Thanawy Backend - Service Starter  " -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""

# Check if Docker is running
Write-Host "[1/4] Checking Docker Desktop..." -ForegroundColor Yellow
try {
    $dockerInfo = docker info 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  ✓ Docker Desktop is running" -ForegroundColor Green
    } else {
        throw "Docker not running"
    }
} catch {
    Write-Host "  ✗ Docker Desktop is NOT running" -ForegroundColor Red
    Write-Host "  → Starting Docker Desktop..." -ForegroundColor Yellow
    Start-Process "C:\Program Files\Docker\Docker\Docker Desktop.exe"
    Write-Host "  → Waiting for Docker to start (60 seconds)..." -ForegroundColor Yellow
    Start-Sleep -Seconds 60
    
    # Verify Docker started
    $dockerInfo = docker info 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  ✗ Docker failed to start. Please start it manually." -ForegroundColor Red
        exit 1
    }
    Write-Host "  ✓ Docker Desktop started successfully" -ForegroundColor Green
}

# Navigate to backend directory
Write-Host ""
Write-Host "[2/4] Navigating to backend directory..." -ForegroundColor Yellow
Set-Location "d:\backend"
Write-Host "  ✓ Current directory: $(Get-Location)" -ForegroundColor Green

# Check if .env.docker exists
Write-Host ""
Write-Host "[3/4] Checking configuration..." -ForegroundColor Yellow
if (-not (Test-Path ".env.docker")) {
    Write-Host "  ⚠ .env.docker not found, using default .env" -ForegroundColor Yellow
    $envFile = ".env"
} else {
    Write-Host "  ✓ Using .env.docker configuration" -ForegroundColor Green
    $envFile = ".env.docker"
}

# Stop any existing containers
Write-Host ""
Write-Host "[4/4] Starting services with Docker Compose..." -ForegroundColor Yellow
Write-Host "  → Stopping any existing containers..." -ForegroundColor Yellow
docker compose --env-file $envFile down 2>&1 | Out-Null

Write-Host "  → Starting all services (this may take 2-3 minutes)..." -ForegroundColor Yellow
docker compose --env-file $envFile up -d

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "=====================================" -ForegroundColor Green
    Write-Host "  ✓ All Services Started Successfully! " -ForegroundColor Green
    Write-Host "=====================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Services running:" -ForegroundColor Cyan
    Write-Host "  • PostgreSQL:  localhost:5432" -ForegroundColor White
    Write-Host "  • Redis:       localhost:6379" -ForegroundColor White
    Write-Host "  • MinIO:       localhost:9000 (API), localhost:9001 (Console)" -ForegroundColor White
    Write-Host "  • Backend API: localhost:8082" -ForegroundColor White
    Write-Host "  • Frontend:    localhost:3000" -ForegroundColor White
    Write-Host ""
    Write-Host "Useful commands:" -ForegroundColor Cyan
    Write-Host "  • View logs:   docker compose logs -f backend" -ForegroundColor Gray
    Write-Host "  • Stop all:    docker compose down" -ForegroundColor Gray
    Write-Host "  • Restart:     docker compose restart backend" -ForegroundColor Gray
    Write-Host "  • Check status: docker compose ps" -ForegroundColor Gray
    Write-Host ""
    
    # Show service status
    Write-Host "Service Status:" -ForegroundColor Cyan
    docker compose ps
} else {
    Write-Host ""
    Write-Host "✗ Failed to start services. Check the errors above." -ForegroundColor Red
    exit 1
}
