#requires -Version 5.1
[CmdletBinding()]
param([string]$GoExecutable = 'go')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$source = & (Join-Path $root 'tools\frontend\prepare.ps1')
$source = [string]($source | Select-Object -Last 1)
$local = Join-Path $root '.local'
$envNames = @('COREPACK_HOME','YARN_CACHE_FOLDER','TEMP','TMP','GOTMPDIR','GOMODCACHE','GOCACHE','GOMAXPROCS','npm_package_name','npm_package_version')
$previous = @{}
foreach ($name in $envNames) { $previous[$name] = [Environment]::GetEnvironmentVariable($name,'Process') }
try {
    foreach ($pair in @{
        COREPACK_HOME='corepack'; YARN_CACHE_FOLDER='yarn-cache'; TEMP='temp'; TMP='temp'; GOTMPDIR='temp'; GOMODCACHE='go-mod-cache'; GOCACHE='go-build-cache'
    }.GetEnumerator()) {
        $target=Join-Path $local $pair.Value
        New-Item -ItemType Directory -Force -Path $target | Out-Null
        [Environment]::SetEnvironmentVariable($pair.Key,$target,'Process')
    }
    $env:GOMAXPROCS='2'
    Push-Location -LiteralPath $source
    try {
        & corepack yarn install --frozen-lockfile --ignore-scripts --non-interactive
        if ($LASTEXITCODE -ne 0) { throw 'Frontend dependency installation failed.' }
        $env:npm_package_name='cloudreve-frontend'; $env:npm_package_version='4.19.1'
        & node --max-old-space-size=6144 node_modules/vite/bin/vite.js build
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed; the current trial program was not changed.' }
    } finally { Pop-Location }
    $zip=Join-Path $root 'application\statics\assets.zip'
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $staging=Join-Path $local 'frontend-assets.zip'
    if (Test-Path -LiteralPath $staging) { Remove-Item -LiteralPath $staging }
    $build=Join-Path $source 'build'
    $archive=[IO.Compression.ZipFile]::Open($staging,[IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach($file in Get-ChildItem -LiteralPath $build -File -Recurse) {
            $relative=$file.FullName.Substring($build.Length).TrimStart('\','/').Replace('\','/')
            [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive,$file.FullName,"assets/build/$relative") | Out-Null
        }
    } finally { $archive.Dispose() }
    Copy-Item -LiteralPath $staging -Destination $zip -Force
    Push-Location -LiteralPath $root
    try {
        & $GoExecutable test -p 2 ./pkg/filemanager/fs/dbfs ./pkg/filemanager/driver/imagebed ./pkg/filemanager/manager/entitysource ./service/admin
        if ($LASTEXITCODE -ne 0) { throw 'Focused backend tests failed.' }
        $output=Join-Path $local 'build\cloudreve-imagebed.exe'
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $output) | Out-Null
        $flags='-s -w -X github.com/cloudreve/Cloudreve/v4/application/constants.BackendVersion=4.19.1 -X github.com/cloudreve/Cloudreve/v4/application/constants.LastCommit=a8becb9-imagebed-local'
        # Uncommitted local assets must not reuse the base commit's Last-Modified.
        # Without VCS metadata Cloudreve uses startup time for embedded resources,
        # so a browser can detect the rebuilt service worker instead of receiving 304.
        & $GoExecutable build -buildvcs=false -p 2 -ldflags $flags -o $output .
        if ($LASTEXITCODE -ne 0) { throw 'Backend build failed.' }
        Write-Output "Built $output. The running official trial binary is unchanged."
    } finally { Pop-Location }
}
finally { foreach ($name in $envNames) { [Environment]::SetEnvironmentVariable($name,$previous[$name],'Process') } }
