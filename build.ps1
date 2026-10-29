param([string]$Go = 'go', [string]$Bash = 'bash', [string]$Python = 'python')
$ErrorActionPreference = 'Stop'
& $Python (Join-Path $PSScriptRoot 'tools/build.py') --go $Go --bash $Bash
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
