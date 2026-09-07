param([string]$Context = 'k3d-teko-k8s')
$ErrorActionPreference = 'Stop'
Write-Host 'Ein Pod bekommt 150 Sekunden kuenstliche CPU-Last. HPA in einem zweiten Terminal beobachten.'
kubectl --context $Context -n betrieb-lab exec deployment/lab-web -- timeout 150 sh -c 'while true; do true; done'
if ($LASTEXITCODE -notin @(0, 124, 143)) { throw 'Lasttest fehlgeschlagen.' }
Write-Host 'Last beendet. Der HPA skaliert nach seiner Wartezeit wieder herunter.'
