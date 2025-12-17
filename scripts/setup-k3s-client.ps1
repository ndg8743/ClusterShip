# ClusterShip k3s Client Setup Script
# Run this on Windows after setting up k3s on Mac

param(
    [Parameter(Mandatory=$true)]
    [string]$MacIP,

    [Parameter(Mandatory=$false)]
    [string]$KubeconfigPath = "$env:USERPROFILE\.kube\config-clustership"
)

Write-Host "ClusterShip Multi-Node Setup" -ForegroundColor Cyan
Write-Host "=============================" -ForegroundColor Cyan
Write-Host ""

# Test connectivity
Write-Host "Testing connection to Mac at $MacIP..." -ForegroundColor Yellow
$ping = Test-Connection -ComputerName $MacIP -Count 1 -Quiet
if (-not $ping) {
    Write-Host "ERROR: Cannot reach $MacIP. Check that:" -ForegroundColor Red
    Write-Host "  1. Both machines are on the same network" -ForegroundColor Red
    Write-Host "  2. Mac firewall allows connections on port 6443" -ForegroundColor Red
    exit 1
}
Write-Host "Connection successful!" -ForegroundColor Green
Write-Host ""

# Test k3s API port
Write-Host "Testing k3s API port (6443)..." -ForegroundColor Yellow
$tcpTest = Test-NetConnection -ComputerName $MacIP -Port 6443 -WarningAction SilentlyContinue
if (-not $tcpTest.TcpTestSucceeded) {
    Write-Host "WARNING: Port 6443 not reachable. On Mac, run:" -ForegroundColor Yellow
    Write-Host "  sudo ufw allow 6443/tcp  # if using ufw" -ForegroundColor Cyan
    Write-Host "  # Or check System Preferences > Security > Firewall" -ForegroundColor Cyan
}
else {
    Write-Host "k3s API port is accessible!" -ForegroundColor Green
}
Write-Host ""

# Instructions
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "1. On Mac, run: sudo cat /etc/rancher/k3s/k3s.yaml" -ForegroundColor White
Write-Host "2. Copy the output" -ForegroundColor White
Write-Host "3. Save it to: $KubeconfigPath" -ForegroundColor White
Write-Host "4. Edit the file and replace '127.0.0.1' with '$MacIP'" -ForegroundColor White
Write-Host ""
Write-Host "Then test with:" -ForegroundColor Cyan
Write-Host "  kubectl --kubeconfig=$KubeconfigPath get nodes" -ForegroundColor White
Write-Host ""
Write-Host "For ClusterShip, update settings:" -ForegroundColor Cyan
Write-Host "  Kubeconfig path: $KubeconfigPath" -ForegroundColor White
Write-Host "  Enable Real K8s: ON" -ForegroundColor White
