$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$BinDir = Join-Path $RootDir "bin"
$GoBin = if ($env:GO_BIN) { $env:GO_BIN } else { "go" }

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

& $GoBin build -o (Join-Path $BinDir "codex-feishu-link.exe") (Join-Path $RootDir "cmd/codex-feishu-link")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$installArgs = @($args)
if ($installArgs.Count -eq 0) {
  $installArgs = @("-bootstrap-only", "-start-daemon")
}

& (Join-Path $BinDir "codex-feishu-link.exe") install @installArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
