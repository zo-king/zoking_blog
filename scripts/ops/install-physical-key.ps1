$ErrorActionPreference = "Stop"

$publicKeyPath = Join-Path $env:USERPROFILE ".ssh\id_ed25519.pub"
if (-not (Test-Path -LiteralPath $publicKeyPath -PathType Leaf)) {
    throw "SSH public key not found: $publicKeyPath"
}

Get-Content -LiteralPath $publicKeyPath |
    ssh -o IdentitiesOnly=yes zoking@192.168.0.21 'umask 077; mkdir -p ~/.ssh; cat >> ~/.ssh/authorized_keys; chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys'

if ($LASTEXITCODE -ne 0) {
    throw "SSH key installation failed"
}

Write-Host "Physical server SSH key installed. This window can now be closed." -ForegroundColor Green
