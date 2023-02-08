#build.ps1
#
# This is a simplistic build tool that runs under PowerShell to
# build the Windows version of Ego, injecting the version number
# from the buildver.txt file.

$vers = Get-Content -Path .\tools\buildver.txt -Raw

Write-Host "Building Ego $vers (Windows)"

go build -ldflags "-X main.BuildVersion=$vers"
