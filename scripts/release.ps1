# Builds every release asset and publishes a GitHub release.
#
#   .\scripts\release.ps1 -Version 0.3.0
#
# Prereqs: go, gh (authenticated), JDK 17+. Run from anywhere; paths are
# anchored to the repo root. The version must match build.gradle.kts.

param(
    [Parameter(Mandatory = $true)][string]$Version,
    [string]$Notes = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# Guard: gradle version must match, or the zip name won't line up.
$gradle = Get-Content "$root\jetbrains\build.gradle.kts" -Raw
if ($gradle -notmatch [regex]::Escape("version = `"$Version`"")) {
    Write-Error "build.gradle.kts version does not match $Version — bump it first."
}

$rel = "$root\release"
New-Item -ItemType Directory -Force $rel | Out-Null
Remove-Item "$rel\*" -Force -ErrorAction SilentlyContinue

Write-Host "== plugin (clean build — incremental ships stale jars) =="
Push-Location "$root\jetbrains"
.\gradlew clean buildPlugin
Pop-Location
Copy-Item "$root\jetbrains\build\distributions\pith-jetbrains-$Version.zip" $rel

Write-Host "== cli binaries =="
$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Out = "pith-windows-amd64.exe" },
    @{ GOOS = "linux";   GOARCH = "amd64"; Out = "pith-linux-amd64" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; Out = "pith-darwin-arm64" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; Out = "pith-darwin-amd64" }
)
Push-Location $root
foreach ($t in $targets) {
    $env:GOOS = $t.GOOS; $env:GOARCH = $t.GOARCH
    go build -ldflags "-s -w" -o "$rel\$($t.Out)" ./cmd/pith
    Write-Host "   $($t.Out)"
}
$env:GOOS = ""; $env:GOARCH = ""
Pop-Location

Write-Host "== github release v$Version =="
if ($Notes -eq "") { $Notes = "pith $Version — see commit history for changes." }
$assets = Get-ChildItem $rel | ForEach-Object { $_.FullName }
Push-Location $root
gh release create "v$Version" @assets --title "pith $Version" --notes $Notes
Pop-Location
