# Builds every release asset and publishes a GitHub release.
#
#   .\scripts\release.ps1 -Version 0.3.0
#
# Prereqs: go, gh (authenticated), JDK 17+, node/npx (for the VS Code vsix).
# Run from anywhere; paths are anchored to the repo root. The version must
# match build.gradle.kts, vscode/package.json, and nvim M.version.
# ASCII only in this file: PS 5.1 reads BOM-less .ps1 as ANSI.

param(
    [Parameter(Mandatory = $true)][string]$Version,
    [string]$Notes = ""
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# Guards: every version pin must match, or assets ship with the wrong tag baked in.
$gradle = Get-Content "$root\jetbrains\build.gradle.kts" -Raw
if ($gradle -notmatch [regex]::Escape("version = `"$Version`"")) {
    Write-Error "build.gradle.kts version does not match $Version - bump it first."
}
$vspkg = Get-Content "$root\vscode\package.json" -Raw
if ($vspkg -notmatch [regex]::Escape("`"version`": `"$Version`"")) {
    Write-Error "vscode/package.json version does not match $Version - bump it first."
}
$nvimlua = Get-Content "$root\nvim\lua\pith\init.lua" -Raw
if ($nvimlua -notmatch [regex]::Escape("M.version = `"$Version`"")) {
    Write-Error "nvim init.lua M.version does not match $Version - bump it first (it pins the auto-download)."
}

$rel = "$root\release"
New-Item -ItemType Directory -Force $rel | Out-Null
Remove-Item "$rel\*" -Force -ErrorAction SilentlyContinue

Write-Host "== cli binaries =="
$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Out = "pith-windows-amd64.exe" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Out = "pith-windows-arm64.exe" },
    @{ GOOS = "linux";   GOARCH = "amd64"; Out = "pith-linux-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Out = "pith-linux-arm64" },
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

Write-Host "== jetbrains plugin (clean build - incremental ships stale jars; bundles all platforms) =="
Push-Location "$root\jetbrains"
.\gradlew clean buildPlugin "-PbundlePlatforms=all"
Pop-Location
Copy-Item "$root\jetbrains\build\distributions\pith-jetbrains-$Version.zip" $rel

Write-Host "== neovim plugin zip (lua only - the plugin downloads its pinned binary) =="
Compress-Archive -Path "$root\nvim\*" -DestinationPath "$rel\pith-nvim-$Version.zip" -Force

Write-Host "== vscode extension (bundles all platform binaries) =="
$vsbin = "$root\vscode\bin"
New-Item -ItemType Directory -Force $vsbin | Out-Null
Remove-Item "$vsbin\*" -Force -ErrorAction SilentlyContinue
foreach ($t in $targets) { Copy-Item "$rel\$($t.Out)" $vsbin }
Push-Location "$root\vscode"
npx --yes @vscode/vsce package -o "$rel\pith-vscode-$Version.vsix"
if ($LASTEXITCODE -ne 0) { Pop-Location; Write-Error "vsce package failed" }
Pop-Location

Write-Host "== github release v$Version =="
if ($Notes -eq "") { $Notes = "pith $Version - see commit history for changes." }
$assets = Get-ChildItem $rel | ForEach-Object { $_.FullName }
Push-Location $root
gh release create "v$Version" @assets --title "pith $Version" --notes $Notes
Pop-Location
