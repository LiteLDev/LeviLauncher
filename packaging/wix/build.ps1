<#
.SYNOPSIS
Builds the Windows installers of LeviLauncher: LeviLauncher-<Version>-win-<Arch>.msi, and
LeviLauncher-<Version>-win-<Arch>-setup.exe, which also installs the Visual C++ and WebView2
runtimes when they are missing.

.EXAMPLE
./packaging/wix/build.ps1 -Version 1.2.3 -Arch x64 -BinDir bin -OutDir .tmp/wix
#>
# 7.3 for $PSNativeCommandUseErrorActionPreference, without which a failed build goes unnoticed.
#Requires -Version 7.3
param(
    [Parameter(Mandatory)]
    [string] $Version,

    [Parameter(Mandatory)]
    [ValidateSet('x64', 'arm64')]
    [string] $Arch,

    # Directory containing the built LeviLauncher.exe.
    [Parameter(Mandatory)]
    [string] $BinDir,

    [Parameter(Mandatory)]
    [string] $OutDir
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$BinDir = (Resolve-Path $BinDir).Path
$OutDir = New-Item -ItemType Directory -Force $OutDir | Select-Object -ExpandProperty FullName

if (-not (Test-Path (Join-Path $BinDir 'LeviLauncher.exe'))) {
    throw "Cannot find LeviLauncher.exe in $BinDir."
}

# Windows Installer versions are numeric, so drop the SemVer pre-release and build metadata.
$productVersion = $Version -replace '[-+].*$', ''
if ($productVersion -notmatch '^\d+\.\d+\.\d+$') {
    throw "Cannot derive a Windows Installer version from '$Version'."
}
# Burn compares SemVer pre-releases, so that 1.2.3 upgrades the bundle of 1.2.3-beta.1. Build
# metadata does not take part in precedence.
$bundleVersion = $Version -replace '\+.*$', ''

# Microsoft publishes no checksums for these evergreen links, so each download must be a
# program signed by Microsoft instead.
function Get-Installer([string] $Url, [string] $FileName) {
    $downloadDir = New-Item -ItemType Directory -Force (Join-Path $repoRoot ".tmp/wix-download/$Arch")
    $file = Join-Path $downloadDir $FileName
    Write-Host "Downloading $Url"
    Invoke-WebRequest $Url -OutFile "$file.partial" -MaximumRetryCount 5 -RetryIntervalSec 5

    $header = [byte[]]::new(2)
    $stream = [IO.File]::OpenRead("$file.partial")
    try {
        $null = $stream.Read($header, 0, 2)
    } finally {
        $stream.Dispose()
    }
    $signature = Get-AuthenticodeSignature "$file.partial"
    if ([Text.Encoding]::ASCII.GetString($header) -ne 'MZ' -or
        $signature.Status -ne 'Valid' -or
        $signature.SignerCertificate.Subject -notmatch '(^|, )O=Microsoft Corporation(,|$)') {
        Remove-Item "$file.partial"
        throw "$Url is not a program signed by Microsoft (signature: $($signature.Status), $($signature.SignerCertificate.Subject))."
    }

    Move-Item -Force "$file.partial" $file
    $file
}

$vcRuntimeInstaller = Get-Installer "https://aka.ms/vc14/vc_redist.$Arch.exe" "vc_redist.$Arch.exe"
# The bootstrapper is the same for every architecture: it downloads the matching runtime.
$webView2Installer = Get-Installer 'https://go.microsoft.com/fwlink/p/?LinkId=2124703' 'MicrosoftEdgeWebview2Setup.exe'

# Incremental builds would not notice that only the properties below changed.
dotnet build (Join-Path $PSScriptRoot 'Bundle/LeviLauncher.Bundle.wixproj') `
    --configuration Release `
    --no-incremental `
    "-p:Platform=$Arch" `
    "-p:ProductVersion=$productVersion" `
    "-p:BundleVersion=$bundleVersion" `
    "-p:BinDir=$BinDir" `
    "-p:VCRuntimeInstaller=$vcRuntimeInstaller" `
    "-p:WebView2Installer=$webView2Installer"

Copy-Item (Join-Path $PSScriptRoot "Package/bin/$Arch/Release/LeviLauncher.msi") (Join-Path $OutDir "LeviLauncher-$Version-win-$Arch.msi")
Copy-Item (Join-Path $PSScriptRoot "Bundle/bin/$Arch/Release/LeviLauncher-setup.exe") (Join-Path $OutDir "LeviLauncher-$Version-win-$Arch-setup.exe")
