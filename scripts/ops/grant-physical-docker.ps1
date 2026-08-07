$ErrorActionPreference = "Stop"

$privateKeyPath = Join-Path $env:USERPROFILE ".ssh\id_ed25519"
ssh -tt -o IdentitiesOnly=yes -i $privateKeyPath zoking@192.168.0.21 'sudo usermod -aG docker zoking'

if ($LASTEXITCODE -ne 0) {
    throw "Could not add zoking to the docker group"
}

Write-Host "Docker group membership updated. New SSH sessions will use it." -ForegroundColor Green
