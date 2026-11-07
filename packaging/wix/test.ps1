<#
.SYNOPSIS
Installs, upgrades and uninstalls the Windows installers of LeviLauncher on this machine, and
checks the result of each step. Must run elevated, on a disposable machine where LeviLauncher
is not installed and has never run: uninstalling deletes the launcher data in %APPDATA% and
%LOCALAPPDATA%.
Compatible with Windows PowerShell 5.1, so that it also runs in Windows Server Core containers.

.PARAMETER ArtifactsDir
Directory containing LeviLauncher-<version>-win-<Arch>.msi and
LeviLauncher-<version>-win-<Arch>-setup.exe for the versions 0.0.1 and 0.0.2, built from a
LeviLauncher.exe that supports --uninstall-cleanup.

.PARAMETER InnoSetup
A setup.exe of a LeviLauncher version released with the former Inno Setup installer. Required
by the Inno scenario.
#>
param(
    [Parameter(Mandatory)]
    [string] $ArtifactsDir,

    [Parameter(Mandatory)]
    [ValidateSet('x64', 'arm64')]
    [string] $Arch,

    [ValidateSet('Inno', 'Msi', 'Bundle')]
    [string[]] $Scenario = @('Inno', 'Msi', 'Bundle'),

    [string] $InnoSetup
)

$ErrorActionPreference = 'Stop'

$ArtifactsDir = (Resolve-Path $ArtifactsDir).Path
$logDir = New-Item -ItemType Directory -Force (Join-Path $ArtifactsDir 'logs') | Select-Object -ExpandProperty FullName
$installDir = Join-Path $env:ProgramFiles 'LeviMC\LeviLauncher'
$uninstallKey = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall'
$innoUninstallKey = "$uninstallKey\org.levimc.launcher_is1"
$startMenuShortcut = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\LeviLauncher.lnk'
$innoStartMenuFolder = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\LeviLauncher'
# Not $desktopShortcut: PowerShell names are case-insensitive, and Assert-Installed has a $DesktopShortcut parameter.
$desktopShortcutPath = Join-Path $env:PUBLIC 'Desktop\LeviLauncher.lnk'
# The storage layout of the launcher (internal/apppath/README.md), and the legacy one that the
# launchers installed by Inno Setup used.
$configDir = Join-Path $env:APPDATA 'LeviLauncher'
$localDir = Join-Path $env:LOCALAPPDATA 'LeviLauncher'
$legacyDir = Join-Path $env:APPDATA 'levilauncher.exe'
$baseRoot = Join-Path $ArtifactsDir 'base_root'

function Assert([bool] $Condition, [string] $Message) {
    if (-not $Condition) {
        throw "FAILED: $Message"
    }
    Write-Host "ok - $Message"
}

function Write-NewLogs([datetime] $Since) {
    Get-ChildItem $logDir -File |
        Where-Object { $_.LastWriteTime -ge $Since } |
        ForEach-Object {
            Write-Host "----- $($_.Name) (last 40 lines)"
            Get-Content $_.FullName -Tail 40 | Write-Host
        }
}

function Invoke-Installer([string] $Name, [string] $File, [string[]] $Arguments, [int] $ExpectedExitCode = 0) {
    Write-Host "> $Name"
    $start = Get-Date
    $process = Start-Process $File -ArgumentList $Arguments -PassThru
    # Caches the process handle, without which ExitCode is lost once the process exits.
    $null = $process.Handle
    if (-not $process.WaitForExit(10 * 60 * 1000)) {
        & taskkill.exe /PID $process.Id /T /F
        Write-NewLogs $start
        throw "FAILED: $Name did not exit within 10 minutes"
    }
    if ($process.ExitCode -ne $ExpectedExitCode) {
        Write-NewLogs $start
    }
    Assert ($process.ExitCode -eq $ExpectedExitCode) "$Name exits with $ExpectedExitCode (got $($process.ExitCode))"
}

function Get-Artifact([string] $Version, [string] $Suffix) {
    Join-Path $ArtifactsDir "LeviLauncher-$Version-win-$Arch$Suffix"
}

function Invoke-Msi([string] $Action, [string] $Version, [int] $ExpectedExitCode = 0, [string[]] $Properties = @()) {
    $msi = Get-Artifact $Version '.msi'
    $log = Join-Path $logDir "msi-$($Action.TrimStart('/'))-$Version-$(Get-Random).log"
    $arguments = @($Action, "`"$msi`"", '/qn', '/norestart', '/l*v', "`"$log`"") + $Properties
    Invoke-Installer "msiexec $Action LeviLauncher $Version $Properties" 'msiexec.exe' $arguments $ExpectedExitCode
}

function Test-RegistryValue([string] $Key, [string] $Name) {
    $item = Get-ItemProperty $Key -Name $Name -ErrorAction SilentlyContinue
    if ($item) { $item.$Name } else { $null }
}

# The same checks as the DetectConditions of setup.exe.
function Test-VCRuntime {
    $key = "SOFTWARE\Microsoft\VisualStudio\14.0\VC\Runtimes\$Arch"
    (Test-RegistryValue "HKLM:\$key" 'Installed') -eq 1 -or
        (Test-RegistryValue "HKLM:\SOFTWARE\WOW6432Node\$key" 'Installed') -eq 1
}

function Test-WebView2Runtime {
    $version = Test-RegistryValue 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}' 'pv'
    $version -and $version -ne '0.0.0.0'
}

function Invoke-Bundle([string] $Action, [string] $Version, [string[]] $Variables = @()) {
    $bundle = Get-Artifact $Version '-setup.exe'
    $log = Join-Path $logDir "bundle-$($Action.TrimStart('/'))-$Version-$(Get-Random).log"
    $arguments = @($Action, '/quiet', '/norestart', '/log', "`"$log`"") + $Variables
    Invoke-Installer "setup.exe $Action LeviLauncher $Version $Variables" $bundle $arguments
    if ($Action -eq '/install') {
        Assert (Test-VCRuntime) 'the Visual C++ runtime is installed'
        Assert (Test-WebView2Runtime) 'the WebView2 runtime is installed'
    }
}

function Get-LauncherEntries {
    @(Get-ChildItem $uninstallKey |
        Get-ItemProperty |
        Where-Object { $_.DisplayName -like 'LeviLauncher*' })
}

function Assert-Installed([string] $Version, [bool] $DesktopShortcut) {
    Assert (Test-Path (Join-Path $installDir 'LeviLauncher.exe')) 'LeviLauncher.exe is installed'
    Assert (-not (Test-Path (Join-Path $installDir 'unins000.exe'))) 'the Inno Setup uninstaller is not left behind'
    Assert (Test-Path $startMenuShortcut) 'the Start Menu shortcut exists'
    Assert (-not (Test-Path $innoStartMenuFolder)) 'the Start Menu folder of Inno Setup does not exist'
    Assert ((Test-Path $desktopShortcutPath) -eq $DesktopShortcut) "the desktop shortcut exists: $DesktopShortcut"

    $visible = @(Get-LauncherEntries | Where-Object { -not $_.SystemComponent })
    Assert ($visible.Count -eq 1) "Installed Apps lists LeviLauncher once (got $($visible.Count))"
    Assert ($visible[0].DisplayVersion -eq $Version) "Installed Apps lists LeviLauncher $Version (got $($visible[0].DisplayVersion))"
}

function Assert-NotInstalled {
    $leftovers = @(Get-ChildItem $installDir -Recurse -Force -Name -ErrorAction SilentlyContinue)
    Assert (-not (Test-Path $installDir)) "the install directory does not exist (contains: $($leftovers -join ', '))"
    Assert (-not (Test-Path $startMenuShortcut)) 'the Start Menu shortcut does not exist'
    Assert (-not (Test-Path $desktopShortcutPath)) 'the desktop shortcut does not exist'
    $entries = Get-LauncherEntries
    Assert ($entries.Count -eq 0) "Installed Apps does not list LeviLauncher (got $($entries.DisplayName -join ', '))"
}

# Stands in for the data of a launcher that has run, with its data root in $baseRoot.
function New-LauncherData {
    foreach ($folder in 'Instances', 'Backups', 'Cache\Installers') {
        $null = New-Item -ItemType Directory -Force (Join-Path $baseRoot $folder)
    }
    $null = New-Item -ItemType Directory -Force $configDir, (Join-Path $localDir 'WebView2')
    [IO.File]::WriteAllText((Join-Path $configDir 'config.json'), (@{ base_root = $baseRoot } | ConvertTo-Json))
}

# The same in the legacy layout, as a launcher installed by Inno Setup left it.
function New-LegacyLauncherData {
    foreach ($folder in 'versions', 'backups', 'installers') {
        $null = New-Item -ItemType Directory -Force (Join-Path $baseRoot $folder)
    }
    $null = New-Item -ItemType Directory -Force (Join-Path $legacyDir 'EBWebView')
    [IO.File]::WriteAllText((Join-Path $legacyDir 'config.json'), (@{ base_root = $baseRoot } | ConvertTo-Json))
}

function Remove-LauncherData {
    Remove-Item -Recurse -Force $configDir, $localDir, $legacyDir, $baseRoot -ErrorAction SilentlyContinue
}

foreach ($dir in $configDir, $localDir, $legacyDir) {
    Assert (-not (Test-Path $dir)) "LeviLauncher has never run for this user ($dir does not exist)"
}
Assert-NotInstalled

if ($Scenario -contains 'Inno') {
    Write-Host '# Upgrade from an Inno Setup installation'
    Assert ([bool] $InnoSetup) 'an Inno Setup installer is given'
    Invoke-Installer 'Inno Setup install' $InnoSetup @('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', "/LOG=`"$logDir\inno.log`"")
    Assert (Test-Path $innoUninstallKey) 'the Inno Setup installation is registered'
    Assert (Test-Path (Join-Path $installDir 'unins000.exe')) 'the Inno Setup uninstaller exists'
    New-LegacyLauncherData

    Invoke-Msi '/i' '0.0.1'
    Assert (-not (Test-Path $innoUninstallKey)) 'the Inno Setup installation is unregistered'
    Assert (Test-Path (Join-Path $legacyDir 'config.json')) 'the upgrade keeps config.json'
    Assert-Installed '0.0.1' $false

    Invoke-Msi '/x' '0.0.1' 0 @('DELETEINSTALLERS=1')
    Assert-NotInstalled
    Assert (-not (Test-Path $legacyDir)) 'the uninstall deletes the legacy config.json and WebView2 data'
    Assert (-not (Test-Path (Join-Path $baseRoot 'installers'))) 'DELETEINSTALLERS=1 deletes the legacy installers'
    Assert (Test-Path (Join-Path $baseRoot 'versions')) 'the uninstall keeps the legacy game versions'
    Assert (Test-Path (Join-Path $baseRoot 'backups')) 'the uninstall keeps the legacy backups'
    Remove-LauncherData
}

if ($Scenario -contains 'Msi') {
    Write-Host '# MSI install, upgrade, downgrade and uninstall'
    Invoke-Msi '/i' '0.0.1' 0 @('DESKTOPSHORTCUT=1')
    Assert-Installed '0.0.1' $true

    # The upgrade keeps the desktop shortcut of the installed version.
    Invoke-Msi '/i' '0.0.2'
    Assert-Installed '0.0.2' $true

    # ERROR_INSTALL_FAILURE: downgrades are refused.
    Invoke-Msi '/i' '0.0.1' 1603
    Assert-Installed '0.0.2' $true

    Invoke-Msi '/x' '0.0.2'
    Assert-NotInstalled
    Remove-LauncherData
}

if ($Scenario -contains 'Bundle') {
    Write-Host '# setup.exe install, upgrade and uninstall'
    Invoke-Bundle '/install' '0.0.1'
    Assert-Installed '0.0.1' $true

    Invoke-Bundle '/install' '0.0.2'
    Assert-Installed '0.0.2' $true

    New-LauncherData
    Invoke-Bundle '/uninstall' '0.0.2' @('DeleteBackups=1')
    Assert-NotInstalled
    Assert (-not (Test-Path $configDir)) 'the uninstall deletes config.json'
    Assert (-not (Test-Path (Join-Path $localDir 'WebView2'))) 'the uninstall deletes the WebView2 data'
    Assert (-not (Test-Path (Join-Path $baseRoot 'Backups'))) 'DeleteBackups=1 deletes the backups'
    Assert (Test-Path (Join-Path $baseRoot 'Cache\Installers')) 'the uninstall keeps the downloaded installers'
    Assert (Test-Path (Join-Path $baseRoot 'Instances')) 'the uninstall keeps the game instances'
    Remove-LauncherData

    Write-Host '# setup.exe without a desktop shortcut'
    Invoke-Bundle '/install' '0.0.1' @('DesktopShortcut=0')
    Assert-Installed '0.0.1' $false

    # The upgrade keeps the choice of the installed version.
    Invoke-Bundle '/install' '0.0.2'
    Assert-Installed '0.0.2' $false

    Invoke-Bundle '/uninstall' '0.0.2'
    Assert-NotInstalled
    Remove-LauncherData
}

Write-Host 'All installer tests passed.'
