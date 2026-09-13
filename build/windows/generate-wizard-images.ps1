#Requires -Version 7
# Renders the Inno Setup artwork from the existing application and leaf icons.
# Run after either source image changes:
#   pwsh.exe -NoProfile -File build/windows/generate-wizard-images.ps1

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$iconPath = Join-Path $root 'build/appicon.png'
$leafPath = Join-Path $root 'frontend/src/assets/images/ic_leaf_logo.png'
$outputDir = Join-Path $root 'build/windows/inno'

$panelTop = [System.Drawing.Color]::FromArgb(0x2F, 0x56, 0x33)
$panelBottom = [System.Drawing.Color]::FromArgb(0x1B, 0x33, 0x1F)
$cream = [System.Drawing.Color]::FromArgb(0xF6, 0xF3, 0xEA)
$sage = [System.Drawing.Color]::FromArgb(0x7B, 0xA4, 0x6D)

$icon = [System.Drawing.Image]::FromFile($iconPath)
$leaf = [System.Drawing.Bitmap]::FromFile($leafPath)

function New-Canvas([int]$Width, [int]$Height, [bool]$Transparent = $false) {
    $pixelFormat = if ($Transparent) { [System.Drawing.Imaging.PixelFormat]::Format32bppArgb } else { [System.Drawing.Imaging.PixelFormat]::Format24bppRgb }
    $bitmap = New-Object System.Drawing.Bitmap($Width, $Height, $pixelFormat)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $graphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
    return @($bitmap, $graphics)
}

# The frontend mark has generous, asymmetric transparent margins. Measure its
# visible bounds once so every header size has the same optical alignment.
function Get-VisibleBounds([System.Drawing.Bitmap]$Image) {
    $left, $top, $right, $bottom = $Image.Width, $Image.Height, -1, -1
    for ($y = 0; $y -lt $Image.Height; $y++) {
        for ($x = 0; $x -lt $Image.Width; $x++) {
            if ($Image.GetPixel($x, $y).A -gt 0) {
                $left = [Math]::Min($left, $x)
                $top = [Math]::Min($top, $y)
                $right = [Math]::Max($right, $x)
                $bottom = [Math]::Max($bottom, $y)
            }
        }
    }
    if ($right -lt $left) { throw 'The leaf image is empty.' }
    return [System.Drawing.Rectangle]::FromLTRB($left, $top, $right + 1, $bottom + 1)
}

function Write-CenteredText($Graphics, [string]$Text, $Font, $Brush, [int]$Width, [int]$Top) {
    $size = $Graphics.MeasureString($Text, $Font)
    $Graphics.DrawString($Text, $Font, $Brush, [float](($Width - $size.Width) / 2), [float]$Top)
}

function New-WizardImage([int]$Scale, [string]$FileName) {
    $width = 164 * $Scale
    $height = 314 * $Scale
    $canvas = New-Canvas $width $height
    $bitmap, $graphics = $canvas
    try {
        $gradient = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
            (New-Object System.Drawing.Point(0, 0)),
            (New-Object System.Drawing.Point(0, $height)),
            $panelTop, $panelBottom)
        $graphics.FillRectangle($gradient, 0, 0, $width, $height)
        $gradient.Dispose()

        $iconSize = 88 * $Scale
        $graphics.DrawImage($icon, [int](($width - $iconSize) / 2), [int](72 * $Scale), $iconSize, $iconSize)

        $wordmarkFont = New-Object System.Drawing.Font('Segoe UI Semibold', (19 * $Scale), [System.Drawing.FontStyle]::Regular, [System.Drawing.GraphicsUnit]::Pixel)
        $publisherFont = New-Object System.Drawing.Font('Segoe UI', (11 * $Scale), [System.Drawing.FontStyle]::Regular, [System.Drawing.GraphicsUnit]::Pixel)
        $creamBrush = New-Object System.Drawing.SolidBrush($cream)
        $sageBrush = New-Object System.Drawing.SolidBrush($sage)
        try {
            Write-CenteredText $graphics 'LeviLauncher' $wordmarkFont $creamBrush $width (182 * $Scale)
            $graphics.FillRectangle($sageBrush, [int](($width - 40 * $Scale) / 2), [int](216 * $Scale), (40 * $Scale), [Math]::Max(1, $Scale))
            Write-CenteredText $graphics 'LeviMC' $publisherFont $sageBrush $width (280 * $Scale)
        } finally {
            $wordmarkFont.Dispose(); $publisherFont.Dispose(); $creamBrush.Dispose(); $sageBrush.Dispose()
        }
        $bitmap.Save((Join-Path $outputDir $FileName), [System.Drawing.Imaging.ImageFormat]::Bmp)
    } finally {
        $graphics.Dispose(); $bitmap.Dispose()
    }
}

function New-WizardSmallImage([int]$Scale, [string]$FileName) {
    $size = 55 * $Scale
    $canvas = New-Canvas $size $size $true
    $bitmap, $graphics = $canvas
    try {
        $graphics.Clear([System.Drawing.Color]::Transparent)
        $height = 41 * $Scale
        $width = [int][Math]::Round($height * $leafBounds.Width / $leafBounds.Height)
        $destination = [System.Drawing.Rectangle]::new([int](($size - $width) / 2), 7 * $Scale, $width, $height)
        $graphics.DrawImage($leaf, $destination, $leafBounds, [System.Drawing.GraphicsUnit]::Pixel)
        $bitmap.Save((Join-Path $outputDir $FileName), [System.Drawing.Imaging.ImageFormat]::Png)
    } finally {
        $graphics.Dispose(); $bitmap.Dispose()
    }
}

try {
    $leafBounds = Get-VisibleBounds $leaf
    # File suffixes describe source resolution, not Windows DPI. Inno chooses
    # the closest size for the user's font metrics and display scaling.
    foreach ($scale in 1..4) {
        $suffix = if ($scale -eq 1) { '' } else { "-${scale}x" }
        New-WizardImage $scale "wizard-image$suffix.bmp"
        New-WizardSmallImage $scale "wizard-small$suffix.png"
    }
} finally {
    $icon.Dispose()
    $leaf.Dispose()
}

Get-ChildItem -Path $outputDir -Filter 'wizard-*' | ForEach-Object { '{0} ({1:N0} bytes)' -f $_.Name, $_.Length }
