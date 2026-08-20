@echo off
chcp 65001 >nul
title Thanawy Backend - Manual Startup
echo =====================================
echo   Thanawy Backend - Manual Startup
echo =====================================
echo.

:: Check if Redis is available locally
echo [1/5] Checking Redis installation...
where redis-server >nul 2>&1
if %ERRORLEVEL% == 0 (
    echo   ✓ Redis found, starting...
    start "Redis Server" /B redis-server --service-run
    timeout /t 3 /nobreak >nul
) else (
    echo   ✗ Redis not found in PATH
    echo   → Please install Redis or use Docker Compose instead
    echo.
    pause
    exit /b 1
)

:: Check if PostgreSQL is running
echo.
echo [2/5] Checking PostgreSQL...
netstat -ano | findstr :5433 >nul 2>&1
if %ERRORLEVEL% == 0 (
    echo   ✓ PostgreSQL is running on port 5433
) else (
    echo   ✗ PostgreSQL is NOT running on port 5433
    echo   → Please start PostgreSQL service
    echo.
    pause
    exit /b 1
)

:: Start Backend API
echo.
echo [3/5] Starting Backend API...
cd /d d:\backend
if exist bin\api.exe (
    echo   ✓ Starting backend server...
    start "Backend API" /B bin\api.exe
    timeout /t 5 /nobreak >nul
) else (
    echo   ✗ Backend binary not found at bin\api.exe
    echo   → Building from source...
    go build -o bin\api.exe ./cmd/api
    if %ERRORLEVEL% == 0 (
        echo   ✓ Build successful, starting...
        start "Backend API" /B bin\api.exe
        timeout /t 5 /nobreak >nul
    ) else (
        echo   ✗ Build failed
        pause
        exit /b 1
    )
)

:: Verify backend is running
echo.
echo [4/5] Verifying services...
timeout /t 2 /nobreak >nul
netstat -ano | findstr :8082 >nul 2>&1
if %ERRORLEVEL% == 0 (
    echo   ✓ Backend API is running on port 8082
) else (
    echo   ✗ Backend API failed to start
    echo   → Check logs at logs\api_run.err.log
    pause
    exit /b 1
)

:: Summary
echo.
echo [5/5] Service Status Summary
echo =====================================
echo   ✓ Redis:        localhost:6379
echo   ✓ PostgreSQL:   localhost:5433
echo   ✓ Backend API:  localhost:8082
echo =====================================
echo.
echo Your backend is now ready!
echo.
echo Next steps:
echo   1. Start your Next.js frontend (npm run dev in frontend directory)
echo   2. Open http://localhost:3000 in your browser
echo   3. Check backend logs: type logs\api_run.err.log
echo.
pause
