param (
    [string]$Version,
    [string]$waPCRoot = "c:\wapc"
)

Write-Output ""
$ErrorActionPreference = 'stop'

#Escape space of waPCRoot path
$waPCRoot = $waPCRoot -replace ' ', '` '

# Constants
$waPCCliFileName = "wapc.exe"
$waPCCliFilePath = "${waPCRoot}\${waPCCliFileName}"

# GitHub Org and repo hosting waPC CLI
$GitHubOrg = "wapc"
$GitHubRepo = "cli"

# Set Github request authentication for basic authentication.
if ($Env:GITHUB_USER) {
    $basicAuth = [System.Convert]::ToBase64String([System.Text.Encoding]::ASCII.GetBytes($Env:GITHUB_USER + ":" + $Env:GITHUB_TOKEN));
    $githubHeader = @{"Authorization" = "Basic $basicAuth" }
}
else {
    $githubHeader = @{}
}

if ((Get-ExecutionPolicy) -gt 'RemoteSigned' -or (Get-ExecutionPolicy) -eq 'ByPass') {
    Write-Output "PowerShell requires an execution policy of 'RemoteSigned'."
    Write-Output "To make this change please run:"
    Write-Output "'Set-ExecutionPolicy RemoteSigned -scope CurrentUser'"
    break
}

# Change security protocol to support TLS 1.2 / 1.1 / 1.0 - old powershell uses TLS 1.0 as a default protocol
[Net.ServicePointManager]::SecurityProtocol = "tls12, tls11, tls"

# Check if waPC CLI is installed.
if (Test-Path $waPCCliFilePath -PathType Leaf) {
    Write-Warning "waPC is detected - $waPCCliFilePath"
    Invoke-Expression "$waPCCliFilePath --version"
    Write-Output "Reinstalling waPC..."
}
else {
    Write-Output "Installing waPC..."
}

# Create waPC Directory
Write-Output "Creating $waPCRoot directory"
New-Item -ErrorAction Ignore -Path $waPCRoot -ItemType "directory"
if (!(Test-Path $waPCRoot -PathType Container)) {
    throw "Cannot create $waPCRoot"
}

# Get the list of release from GitHub
$releases = Invoke-RestMethod -Headers $githubHeader -Uri "https://api.github.com/repos/${GitHubOrg}/${GitHubRepo}/releases" -Method Get
if ($releases.Count -eq 0) {
    throw "No releases from github.com/wapc/cli repo"
}

# Filter windows binary and download archive
if (!$Version) {
    $windowsAsset = $releases | Where-Object { $_.tag_name -notlike "*rc*" } | Select-Object -First 1 | Select-Object -ExpandProperty assets | Where-Object { $_.name -Like "*windows_amd64.zip" }
    if (!$windowsAsset) {
        throw "Cannot find the windows waPC CLI binary"
    }
    $zipFileUrl = $windowsAsset.url
    $assetName = $windowsAsset.name
} else {
    $assetName = "wapc_windows_amd64.zip"
    $zipFileUrl = "https://github.com/${GitHubOrg}/${GitHubRepo}/releases/download/v${Version}/${assetName}"
}

$zipFilePath = $waPCRoot + "\" + $assetName
Write-Output "Downloading $zipFileUrl ..."

$githubHeader.Accept = "application/octet-stream"
Invoke-WebRequest -Headers $githubHeader -Uri $zipFileUrl -OutFile $zipFilePath
if (!(Test-Path $zipFilePath -PathType Leaf)) {
    throw "Failed to download waPC Cli binary - $zipFilePath"
}

# Extract waPC CLI to $waPCRoot
Write-Output "Extracting $zipFilePath..."
Microsoft.Powershell.Archive\Expand-Archive -Force -Path $zipFilePath -DestinationPath $waPCRoot
if (!(Test-Path $waPCCliFilePath -PathType Leaf)) {
    throw "Failed to download waPC Cli archieve - $zipFilePath"
}

# Check the waPC CLI version
Invoke-Expression "$waPCCliFilePath --version"

# Clean up zipfile
Write-Output "Clean up $zipFilePath..."
Remove-Item $zipFilePath -Force

# Add waPCRoot directory to User Path environment variable
Write-Output "Try to add $waPCRoot to User Path Environment variable..."
$UserPathEnvironmentVar = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPathEnvironmentVar -like '*wapc*') {
    Write-Output "Skipping to add $waPCRoot to User Path - $UserPathEnvironmentVar"
}
else {
    [System.Environment]::SetEnvironmentVariable("PATH", $UserPathEnvironmentVar + ";$waPCRoot", "User")
    $UserPathEnvironmentVar = [Environment]::GetEnvironmentVariable("PATH", "User")
    Write-Output "Added $waPCRoot to User Path - $UserPathEnvironmentVar"
}

Write-Output "`r`nwaPC CLI is installed successfully."
