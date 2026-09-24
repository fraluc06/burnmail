# Burnmail Build Script for Windows (PowerShell)
# Usage: .\build.ps1 [amd64|arm64|all]

param(
    [string]$Platform = "amd64"
)

$BinaryName = "burnmail"
$Version = "1.4.1"

Write-Host "🔨 Building Burnmail v$Version for Windows" -ForegroundColor Cyan
Write-Host ""

function Write-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-Info {
    param([string]$Message)
    Write-Host "$Message" -ForegroundColor Blue
}

function Build-Windows {
    param([string]$Arch)
    
    Write-Info "📦 Building for Windows $Arch..."
    $env:GOOS = "windows"
    $env:GOARCH = $Arch
    $OutputName = "$BinaryName-windows-$Arch.exe"
    go build -ldflags="-s -w -X burnmail/internal/config.Version=$Version" -o $OutputName
    
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Built: $OutputName"
    } else {
        Write-Host "✗ Build failed for $Arch" -ForegroundColor Red
        exit 1
    }
}

Write-Info "📦 Downloading dependencies..."
go mod download
go mod tidy

switch ($Platform.ToLower()) {
    "amd64" {
        Build-Windows "amd64"
    }
    "arm64" {
        Build-Windows "arm64"
    }
    "all" {
        Build-Windows "amd64"
        Build-Windows "arm64"
    }
    default {
        Write-Host "Unknown platform: $Platform" -ForegroundColor Red
        Write-Host "Usage: .\build.ps1 [amd64|arm64|all]" -ForegroundColor Yellow
        exit 1
    }
}

Write-Host ""
Write-Success "🎉 Build complete!"
Write-Host ""
Write-Host "To install globally:" -ForegroundColor Yellow
Write-Host "  Move the exe to C:\Windows\System32\" -ForegroundColor White
Write-Host "  Or add current directory to PATH" -ForegroundColor White
Write-Host ""
Write-Host "To test:" -ForegroundColor Yellow
Write-Host "  .\$BinaryName-windows-amd64.exe g" -ForegroundColor White
Write-Host "  .\$BinaryName-windows-amd64.exe m" -ForegroundColor White
Write-Host "  .\$BinaryName-windows-amd64.exe me" -ForegroundColor White
Write-Host "  .\$BinaryName-windows-amd64.exe d" -ForegroundColor White