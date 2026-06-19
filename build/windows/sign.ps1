# Authenticode-sign Windows binaries/installer. No-op (leaves files unsigned)
# until the signing secrets are configured, so release builds work without them.
#
# Required secrets to activate signing:
#   WINDOWS_CERTIFICATE          base64 of the code-signing .pfx
#   WINDOWS_CERTIFICATE_PASSWORD password for that .pfx
param(
  [Parameter(Mandatory = $true)]
  [string[]]$Paths
)

$ErrorActionPreference = "Stop"

if (-not $env:WINDOWS_CERTIFICATE) {
  Write-Host "No Windows code-signing certificate configured: leaving binaries unsigned (SmartScreen may warn)."
  exit 0
}

$pfx = Join-Path $env:RUNNER_TEMP "cert.pfx"
[IO.File]::WriteAllBytes($pfx, [Convert]::FromBase64String($env:WINDOWS_CERTIFICATE))
$securePwd = ConvertTo-SecureString $env:WINDOWS_CERTIFICATE_PASSWORD -AsPlainText -Force
$cert = Import-PfxCertificate -FilePath $pfx -CertStoreLocation Cert:\CurrentUser\My -Password $securePwd

$signtool = Get-ChildItem "C:\Program Files (x86)\Windows Kits\10\bin\*\x64\signtool.exe" |
  Sort-Object FullName | Select-Object -Last 1
if (-not $signtool) {
  throw "signtool.exe not found"
}

foreach ($p in $Paths) {
  & $signtool.FullName sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 `
    /sha1 $cert.Thumbprint $p
  if ($LASTEXITCODE -ne 0) { throw "signtool failed for $p" }
  Write-Host "Signed $p"
}
