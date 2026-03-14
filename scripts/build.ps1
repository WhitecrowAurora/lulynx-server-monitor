param(
  [string]$OutDir = "dist",
  [string[]]$Targets = @("linux-amd64"),
  [switch]$NoStrip
)

$ErrorActionPreference = "Stop"

function Ensure-Dir([string]$Path) {
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path | Out-Null
  }
}

Ensure-Dir $OutDir

$supportedTargets = @("linux-amd64", "linux-arm64", "windows-amd64")

function Normalize-Targets([string[]]$Raw) {
  $out = @()
  foreach ($v in $Raw) {
    if ($null -eq $v) { continue }
    foreach ($p in ($v -split ",")) {
      $x = $p.Trim()
      if ($x -ne "") { $out += $x }
    }
  }
  if ($out.Count -eq 0) { return @("linux-amd64") }
  return $out
}

$Targets = Normalize-Targets $Targets
foreach ($t in $Targets) {
  if ($supportedTargets -notcontains $t) {
    throw ("Unsupported target: {0}. Supported: {1}" -f $t, ($supportedTargets -join ", "))
  }
}

$ldflags = ""
if (-not $NoStrip) {
  $ldflags = "-s -w"
}

Write-Host ("OutDir: {0}" -f (Resolve-Path $OutDir))
Write-Host ("Targets: {0}" -f ($Targets -join ", "))

foreach ($t in $Targets) {
  $parts = $t.Split("-", 2)
  $goos = $parts[0]
  $goarch = $parts[1]

  Write-Host ""
  Write-Host ("==> Building {0}/{1}" -f $goos, $goarch)

  $env:GOOS = $goos
  $env:GOARCH = $goarch
  $env:CGO_ENABLED = "0"

  $ext = ""
  if ($goos -eq "windows") { $ext = ".exe" }

  $centerOut = Join-Path $OutDir ("tanzhen-center-{0}-{1}{2}" -f $goos, $goarch, $ext)
  $probeOut  = Join-Path $OutDir ("tanzhen-probe-{0}-{1}{2}" -f $goos, $goarch, $ext)

  $centerArgs = @("build", "-trimpath")
  if ($ldflags -ne "") { $centerArgs += @("-ldflags", $ldflags) }
  $centerArgs += @("-o", $centerOut, "./cmd/center")

  $agentArgs = @("build", "-trimpath")
  if ($ldflags -ne "") { $agentArgs += @("-ldflags", $ldflags) }
  $agentArgs += @("-o", $probeOut, "./cmd/agent")

  & go @centerArgs
  & go @agentArgs

  Write-Host ("  OK: {0}" -f $centerOut)
  Write-Host ("  OK: {0}" -f $probeOut)
}

Write-Host ""
Write-Host "Done."
