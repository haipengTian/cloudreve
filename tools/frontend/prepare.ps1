#requires -Version 5.1
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$revision = '19da0fe1ecd40971fafa813983d769fdce41573c'
$directory = Join-Path $root ".local\frontend-source\frontend-$revision"
$patch = Join-Path $PSScriptRoot 'imagebed.patch'

# Use the exact frontend revision pinned by the backend, not the latest branch.
if (-not (Test-Path -LiteralPath (Join-Path $directory 'package.json'))) {
    $archive = Join-Path $root '.local\frontend-source.zip'
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $archive) | Out-Null
    Invoke-WebRequest "https://codeload.github.com/cloudreve/frontend/zip/$revision" -OutFile $archive
    Expand-Archive -LiteralPath $archive -DestinationPath (Split-Path -Parent $directory)
}

$previousCeiling = $env:GIT_CEILING_DIRECTORIES
# Treat the extracted source as a standalone tree. Otherwise git discovers the
# backend repository and may silently skip frontend paths under .local/.
$env:GIT_CEILING_DIRECTORIES = Split-Path -Parent $directory
Push-Location -LiteralPath $directory
try {
    & git apply --reverse --check $patch 2>$null
    if ($LASTEXITCODE -ne 0) {
        & git apply --check $patch
        if ($LASTEXITCODE -ne 0) { throw 'Frontend differs from the pinned base; refusing to overwrite local changes.' }
        & git apply $patch
        if ($LASTEXITCODE -ne 0) { throw 'Unable to apply ImageBed frontend patch.' }
    }
    Write-Output $directory
}
finally { Pop-Location; $env:GIT_CEILING_DIRECTORIES = $previousCeiling }
