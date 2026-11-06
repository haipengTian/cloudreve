#requires -Version 5.1
[CmdletBinding()]
param([switch]$ImageBed)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$runtime = Join-Path $root '.local\runtime'
$executable = Join-Path $runtime 'cloudreve.exe'
if ($ImageBed) { $executable = Join-Path $root '.local\build\cloudreve-imagebed.exe' }
$config = Join-Path $runtime 'data\conf.ini'

if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
    throw "Cloudreve program is missing: $executable. See docs/LOCAL_TRIAL.md and docs/IMAGEBED_INTEGRATION.md."
}

if (Get-NetTCPConnection -State Listen -LocalPort 5212 -ErrorAction SilentlyContinue) {
    throw 'Port 5212 is already in use. Do not stop another application; check http://127.0.0.1:5212 first.'
}

if ($ImageBed) {
    # Cloudreve resolves its data relative to the executable, not the shell CWD.
    # Keep the candidate beside the official program without replacing it.
    $candidate = Join-Path $runtime 'cloudreve-imagebed.exe'
    Copy-Item -LiteralPath $executable -Destination $candidate -Force
    $executable = $candidate
}

# The upstream program creates its own configuration and random session secret.
# Override only the listen address so this trial is never exposed to the LAN.
$key = 'CR_CONF_System.Listen'
$previousListen = [Environment]::GetEnvironmentVariable($key, 'Process')
try {
    [Environment]::SetEnvironmentVariable($key, '127.0.0.1:5212', 'Process')
    Push-Location -LiteralPath $runtime
    try {
        Write-Host 'Cloudreve local trial: http://127.0.0.1:5212'
        Write-Host 'Keep this window open. Press Ctrl+C to stop; stored files are retained.'
        & $executable server --conf $config
        if ($LASTEXITCODE -ne 0) {
            throw "Cloudreve exited with code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
}
finally {
    [Environment]::SetEnvironmentVariable($key, $previousListen, 'Process')
}
