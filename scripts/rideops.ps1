$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location (Join-Path $root "backend")
try {
  go run . ops @args
} finally {
  Pop-Location
}
