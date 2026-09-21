# Dispatch City - Verteilte Systeme, Containerisierung

Transferarbeit VSC-01 · TEKO Schweizerische Fachschule · Sommer/Herbst 2026

**Autoren:** Cedric Widmer / Cedric-W (Xabeba) · Melih Serit / Mescott01

---

## KI-Hilfsmittel

Claude (Anthropic) wurde als KI-Assistent eingesetzt für:
- Erklärungen zu Kubernetes, Docker, Helm und RabbitMQ
- Debugging (k3d WSL2-Netzwerkprobleme, kubectl-Versionskompatibilität)
- Erstellung dieses README

Alle Befehle wurden selbst ausgeführt und verstanden. Die Lösung ist eigenständig mit Hilfe der gegebenen Arbeitsblätter erarbeitet.

---

## Voraussetzungen

- Docker Desktop (Windows: WSL2-Integration aktiviert)
- k3d v5.9.0
- kubectl v1.32+ – **Wichtig:** Docker Desktop liefert kubectl v1.30 mit, das Multi-Doc-Patches nicht unterstützt. Die Datei `C:\tools\kubectl.exe` muss im PATH vor dem Docker-Desktop-kubectl stehen.
- Helm v3.17+
- Git

PATH korrekt setzen (einmalig pro PowerShell-Session):

```powershell
$env:Path = "C:\tools;" + $env:Path
```

---

## Cluster erstellen

```powershell
k3d cluster create teko-k8s --agents 2 --image rancher/k3s:v1.34.8-k3s1 -p "8080:80@loadbalancer"
```

Port aus `docker ps --filter "name=serverlb"` ablesen (Spalte PORTS, `->6443/tcp`), dann:

```powershell
kubectl config set-cluster k3d-teko-k8s --server=https://127.0.0.1:<PORT>
kubectl get nodes
```

---

## Build

```powershell
# Go-Services und Dashboard bauen
./scripts/build-images.ps1

# CloudNativePG Operator installieren (Block 6)
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
./platform/cloudnative-pg/install.ps1

# Monitoring installieren und starten (Block 7)
./platform/monitoring/start-course.ps1
```

---

## Deploy

```powershell
# Images in k3d-Cluster importieren
./scripts/load-images.ps1 -Cluster teko-k8s

# Gesamten Stack deployen (Block 7 beinhaltet alle vorherigen Blöcke via Kustomize)
kubectl apply -k deploy/overlays/block-07-observability

# Warten bis PostgreSQL-Cluster bereit ist
kubectl -n food-delivery wait --for=condition=Ready cluster/food-delivery-db --timeout=5m
```

---

## Smoke Test

```powershell
# Pod-Status prüfen (alle sollten Running/Ready sein)
kubectl -n food-delivery get pods

# HTTP-Endpunkte prüfen (alle sollen 200 zurückgeben)
(Invoke-WebRequest -UseBasicParsing http://localhost:8080/).StatusCode
(Invoke-WebRequest -UseBasicParsing http://localhost:8080/api/v1/snapshot).StatusCode
(Invoke-WebRequest -UseBasicParsing http://localhost:8080/health/ready).StatusCode

# PostgreSQL-Cluster prüfen
kubectl -n food-delivery get cluster food-delivery-db
```

---

## Demo öffnen

**Dispatch City (Stadt):**
Browser: http://localhost:8080

**Grafana Monitoring:**

```powershell
kubectl -n monitoring port-forward service/monitoring-grafana 3000:80
```

Browser: http://localhost:3000 · Login: `admin` / `delivery` · Dashboard: *Dispatch City - Betrieb*

**RabbitMQ Management:**

```powershell
kubectl -n food-delivery port-forward service/rabbitmq 15672:15672
```

Browser: http://localhost:15672 · Login: `delivery` / `delivery`

---

## Reset

```powershell
# Nur Namespace löschen (Cluster bleibt erhalten)
kubectl delete namespace food-delivery

# Vollständig zurücksetzen
k3d cluster delete teko-k8s
```

---

## Architektur

### Komponenten

| Komponente | Technologie | Aufgabe |
|---|---|---|
| dashboard | Nuxt + PixiJS | 2.5D-Stadt, Live-Events via SSE |
| control-api | Go | REST/SSE/Health/Metrics, In-Memory-Simulation |
| customer-simulator | Go | Erzeugt Kundenidentitäten und Bestellungen als Events |
| restaurant-worker | Go | Verarbeitet Bestellungen als Competing Consumers |
| courier-simulator | Go | Simuliert Kurier-Fahrten und Zustellungen |
| order-worker | Go | Persistiert Events idempotent in PostgreSQL |
| RabbitMQ | Standard-Image | Event-Queue, DLQ, Exchange/Routing |
| PostgreSQL | CloudNativePG | Primary/Standby-Cluster, persistente Datenhaltung |
| Prometheus/Grafana | kube-prometheus-stack | Metriken, Dashboards, HPA |

### Eventfluss

```
customer-simulator
      |
      v (order.created)
   RabbitMQ
      |
      v
restaurant-worker
      |
      v (order.accepted)
   RabbitMQ
      |
      v
courier-simulator
      |
      v (order.delivered)
   RabbitMQ
      |
      v
order-worker --> PostgreSQL
      |
      v
control-api (SSE) --> dashboard
```

### Kustomize-Overlays pro Ausbaustufe

| Block | Overlay | Neue Inhalte |
|---|---|---|
| 3 | block-03-standalone | Basis-Deployment, ConfigMap, Services, Probes |
| 4 | block-04-ingress | Traefik-Ingress, 2 Dashboard-Replicas |
| 5 | block-05-messaging | RabbitMQ, alle Worker-Services, StatefulSets |
| 6 | block-06-persistence | CloudNativePG, Migration-Job, DB-Secrets |
| 7 | block-07-observability | Prometheus, Grafana, HPA, ServiceMonitors, cluster-observer |

---

## Wichtigste Entscheidungen

**k3d auf Windows/WSL2:** Neuere k3s-Images schlagen bei Multi-Node-Clustern fehl (Agent-Registrierung: `context deadline exceeded`). Lösung: `rancher/k3s:v1.34.8-k3s1` und explizites Port-Mapping `-p "8080:80@loadbalancer"`.

**kubectl v1.32 statt Docker-Desktop-Version:** Docker Desktop bringt kubectl v1.30 mit Kustomize v5.0.4 mit, das keine Multi-Doc-Patches unterstützt. `C:\tools\kubectl.exe` (v1.32) muss im PATH vorne stehen.

**control-api bleibt 1 Replica:** Die control-api hält den In-Memory-Simulationszustand. Mehrere Replicas würden inkonsistente Zustände und unterschiedliche SSE-Streams erzeugen.

**food-delivery-db-rw als Datenbank-Host:** Stabiler Kubernetes-Service-Name, der automatisch auf den aktuellen Primary zeigt. Pod-Namen ändern sich bei Failover und wären als Hostname unbrauchbar.

---

## Bekannte Grenzen und Fehlerbilder

- **WSL2 Multi-Node:** k3d mit neueren k3s-Images und `--agents 2` schlägt mit `context deadline exceeded` fehl. Fix: älteres k3s-Image.
- **kubectl PATH-Priorität:** Nach jedem PowerShell-Neustart muss `$env:Path = "C:\tools;" + $env:Path` gesetzt werden, da Docker Desktop sein kubectl bevorzugt.
- **Port-Forward statt LoadBalancer-IP:** Lokale Demo erfordert manuelle Port-Forwards; in Produktion würde ein externer LoadBalancer verwendet.
- **Kein Backup/RPO-Test:** Der Failover-Test beweist automatischen Primary-Wechsel, aber kein Point-in-Time-Recovery oder Backup-Szenario wurde getestet.
- **HPA-Verzögerung:** CPU-Metriken brauchen ~60s bis der HPA reagiert; kurze Lastspitzen werden möglicherweise nicht rechtzeitig erkannt.

---

## Aufgabenverteilung (Partnerarbeit)

| Block | Thema | Cedric / Cedric-W | Melih / Mescott01 |
|---|---|---|---|
| 3 | Kubernetes Foundation | Erarbeitet, deployed, committed | Review |
| 4 | Ingress und Load Balancing | Erarbeitet, deployed, committed | Review |
| 5 | RabbitMQ Event Pipeline | Review | Erarbeitet, deployed, committed |
| 6 | CloudNativePG Persistenz | Review | Erarbeitet, deployed, committed |
| 7 | Observability und Resilienz | Erarbeitet, deployed, committed | Review |
| README / Dokumentation | - | Überarbeitet | Erstellt |
