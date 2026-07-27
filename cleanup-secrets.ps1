# Security Incident Response Automation
# This script helps clean up .env files locally and update .gitignore

param(
    [Parameter(Mandatory=$false)]
    [string]$RepoPath = (Get-Location).Path,
    
    [switch]$Force,
    [switch]$DryRun
)

$DebugPreference = "Continue"
Write-Host "🔒 Security Incident Response Script" -ForegroundColor Cyan
Write-Host "=====================================`n" -ForegroundColor Cyan

# Step 1: Backup .env files (for reference only)
Write-Host "📦 Step 1: Backing up .env files..." -ForegroundColor Yellow
$backupDir = "$RepoPath\.env_backup_$(Get-Date -Format 'yyyyMMdd_HHmmss')"

if (-not (Test-Path $backupDir)) {
    New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
}

$envFiles = Get-ChildItem -Path $RepoPath -Name ".env*" -ErrorAction SilentlyContinue
if ($envFiles) {
    foreach ($file in $envFiles) {
        $srcPath = Join-Path $RepoPath $file
        $dstPath = Join-Path $backupDir $file
        Copy-Item -Path $srcPath -Destination $dstPath -Force
        Write-Host "  ✓ Backed up: $file → $backupDir" -ForegroundColor Green
    }
} else {
    Write-Host "  ℹ No .env files found to backup" -ForegroundColor Gray
}

# Step 2: Remove .env files locally
Write-Host "`n🗑️  Step 2: Removing .env files from working directory..." -ForegroundColor Yellow
$toRemove = Get-ChildItem -Path $RepoPath -Name ".env*" -ErrorAction SilentlyContinue
if ($toRemove) {
    foreach ($file in $toRemove) {
        $filePath = Join-Path $RepoPath $file
        if (-not $DryRun) {
            Remove-Item -Path $filePath -Force
            Write-Host "  ✓ Deleted: $file" -ForegroundColor Green
        } else {
            Write-Host "  [DRY-RUN] Would delete: $file" -ForegroundColor Gray
        }
    }
} else {
    Write-Host "  ℹ No .env files to remove" -ForegroundColor Gray
}

# Step 3: Remove .next, .kilo, and other build artifacts
Write-Host "`n🗑️  Step 3: Removing build artifacts..." -ForegroundColor Yellow
$buildArtifacts = @(".next", ".kilo", ".vercel", "dist", "build", "api", "bin/api")
foreach ($artifact in $buildArtifacts) {
    $artifactPath = Join-Path $RepoPath $artifact
    if (Test-Path $artifactPath) {
        if (-not $DryRun) {
            Remove-Item -Path $artifactPath -Recurse -Force
            Write-Host "  ✓ Deleted: $artifact/" -ForegroundColor Green
        } else {
            Write-Host "  [DRY-RUN] Would delete: $artifact/" -ForegroundColor Gray
        }
    }
}

# Step 4: Remove test output and logs
Write-Host "`n🗑️  Step 4: Removing logs and debug files..." -ForegroundColor Yellow
$debugFiles = @("test_output.txt", "*_error.log", "*_output.log", "*_run.log", "findstr", "inspect_routes.py")
foreach ($pattern in $debugFiles) {
    $matches = Get-ChildItem -Path $RepoPath -Name $pattern -ErrorAction SilentlyContinue
    if ($matches) {
        foreach ($file in $matches) {
            $filePath = Join-Path $RepoPath $file
            if (-not $DryRun) {
                Remove-Item -Path $filePath -Force -ErrorAction SilentlyContinue
                Write-Host "  ✓ Deleted: $file" -ForegroundColor Green
            } else {
                Write-Host "  [DRY-RUN] Would delete: $file" -ForegroundColor Gray
            }
        }
    }
}

# Step 5: Update/Create .gitignore
Write-Host "`n📝 Step 5: Updating .gitignore..." -ForegroundColor Yellow
$gitignorePath = Join-Path $RepoPath ".gitignore"
$newGitignoreContent = @"
# Environment files - NEVER commit
.env
.env.local
.env.*.local
.env.production*
.env.development*
.env.test
.env.*.pulled
.env.vercel*
.env.*.secret
secrets/

# Build artifacts & executables
.next/
dist/
build/
bin/
api/
*.exe
api.exe
migrate.exe
test*.exe
main.exe
app.exe
*.tsbuildinfo
TS_NODE_*

# AI / Automation
.kilo/
.agents/
.vercel/
.vercelignore

# Node & Go
node_modules/
vendor/
package-lock.json.bak
go.sum.bak

# IDE & Editor
.vscode/
.idea/
*.swp
*.swo
*~
.DS_Store

# Secrets & keys
*.key
*.pem
*.crt
secrets.yaml
secrets.json
.secrets/

# Compiled files
*.so
*.dylib
*.o

# Logs & test output
*.log
logs/
npm-debug.log*
yarn-debug.log*
yarn-error.log*
test_output.txt
*_error.log
*_output.log
*_run.log
server.log

# Temporary & debug files
.cache/
.tmp/
temp/
findstr
inspect_routes.py
_scratch/
*.bak

# OS files
Thumbs.db
.DS_Store

# Development/Debug
.env.check
.env.development.pulled
.env.production.pulled
skills-lock.json
tsconfig.tsbuildinfo
"@

if (-not $DryRun) {
    Set-Content -Path $gitignorePath -Value $newGitignoreContent -Force
    Write-Host "  ✓ Created/Updated .gitignore" -ForegroundColor Green
} else {
    Write-Host "  [DRY-RUN] Would update .gitignore with security patterns" -ForegroundColor Gray
}

# Step 6: Stage .gitignore change for git
Write-Host "`n📤 Step 6: Staging changes..." -ForegroundColor Yellow
if (-not $DryRun) {
    Push-Location $RepoPath
    git add .gitignore 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  ✓ Staged .gitignore" -ForegroundColor Green
    } else {
        Write-Host "  ⚠️  Git not available or error staging" -ForegroundColor Yellow
    }
    Pop-Location
}

# Step 7: Show git status
Write-Host "`n📋 Step 7: Current status..." -ForegroundColor Yellow
if (-not $DryRun) {
    Push-Location $RepoPath
    Write-Host "`nGit Status:" -ForegroundColor Cyan
    git status --short 2>$null | Where-Object { $_ -match '\.env|\.next|\.kilo' } | ForEach-Object {
        Write-Host "  $_" -ForegroundColor Gray
    }
    
    # Check for any remaining secrets in tracked files
    Write-Host "`n🔍 Checking for potential secrets in tracked files..." -ForegroundColor Yellow
    $suspiciousPatterns = @(
        "SECRET",
        "PASSWORD",
        "APIKEY",
        "api_key",
        "sk-or-v1",     # OpenRouter pattern
        "GOCSPX",       # Google OAuth pattern
        "xsmtpsib",     # Brevo SMTP pattern
        "CG1xOpG2d2K9"  # Old Redis password start
    )
    
    $foundSecrets = 0
    foreach ($pattern in $suspiciousPatterns) {
        $matches = git grep -i $pattern 2>$null | Where-Object { $_ -notmatch "SECURITY|\.md:|node_modules|vendor" }
        if ($matches) {
            Write-Host "  ⚠️  Pattern '$pattern' found in:" -ForegroundColor Yellow
            $matches | ForEach-Object { Write-Host "     $_" -ForegroundColor Gray }
            $foundSecrets++
        }
    }
    
    if ($foundSecrets -eq 0) {
        Write-Host "  ✓ No obvious secrets detected in tracked files" -ForegroundColor Green
    }
    
    Pop-Location
}

# Summary
Write-Host "`n" -ForegroundColor Cyan
Write-Host "════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "✅ Cleanup Complete!" -ForegroundColor Green
Write-Host "════════════════════════════════════════" -ForegroundColor Cyan
Write-Host @"

📌 NEXT STEPS (MANUAL):

1. ✋ STOP HERE - Don't push yet!

2. 🔑 Rotate ALL credentials in external services:
   - Supabase Database Password
   - RedisLabs Password
   - Generate NEW JWT_SECRET
   - Google OAuth (revoke old, create new)
   - OpenRouter API Key (revoke old, create new)
   - Brevo SMTP (create new user, disable old)
   - VAPID Keys (generate new pair)

3. 📄 Create NEW .env files with ONLY NEW credentials:
   cp .env.example .env
   # Edit with new credentials only

4. 🧹 Use BFG Repo-Cleaner to remove from git history:
   # MANUAL: Run commands in SECURITY_INCIDENT_ACTION_PLAN.md

5. 🔒 Make repositories PRIVATE on GitHub immediately

6. 🚀 Re-deploy with NEW credentials

⏰ BACKUP LOCATION: $backupDir
   (Save this for your records, then delete when secure)

📖 FULL INSTRUCTIONS: See admin\..\SECURITY_INCIDENT_ACTION_PLAN.md
"@ -ForegroundColor Cyan

if ($DryRun) {
    Write-Host "`n⚠️  DRY-RUN MODE: No files were actually changed" -ForegroundColor Yellow
    Write-Host "   Run without -DryRun to apply changes`n" -ForegroundColor Yellow
}
