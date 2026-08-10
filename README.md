# Block 3 - Kubernetes Foundation

Dieser Baustein ist der lauffaehige fachliche Startpunkt: Eine Go-API simuliert Restaurants, Kunden, Kuriere und Bestellungen im Speicher. Das Nuxt-Dashboard stellt von Anfang an eine interaktive 21x21-Tile-Stadt mit Strassen, Gebaeuden, Parks und animierten Fahrtrouten dar.

## Verwendung im Kurs

- Oeffentliche Grundlage fuer Arbeitsblatt 3 und als GitHub-Template vorbereitet
- Pro Einzelperson oder Zweierteam entsteht daraus genau ein eigenes Repository
- Dieses Repository wird in den folgenden Unterrichtsbloecken fortlaufend erweitert
- Fuer einen reproduzierbaren Start ist der Release `v1.0.0` zu verwenden

## Enthalten

- `apps/dashboard`: Nuxt 4 und PixiJS fuer die 2.5D-Stadt
- `cmd/control-api`: REST-, SSE-, Health- und Metrics-Endpunkte
- `deploy/base`: Namespace, ConfigMap, Deployments und Services
- `deploy/overlays/block-03-standalone`: erster deploybarer Stand
- Multi-Stage-Dockerfiles und Build-/Import-Skripte

## Arbeitsauftrag

1. Images bauen und deren Layer, Groesse und Tags untersuchen.
2. Deployment, Service, ConfigMap, Ressourcenlimits und Probes nachvollziehen.
3. Den Stand in k3d deployen und die internen DNS-Namen aus einem Debug-Pod testen.
4. `control-api` und `dashboard` skalieren und das Verhalten der Services beobachten.
5. Einen Pod loeschen und Self-Healing sowie Readiness dokumentieren.
6. Im Standalone-Modus beobachten, wie ein Kurier zuerst auf der Strasse zum Restaurant, danach zum Kunden und schliesslich als freie Flottenentitaet an der letzten Position bleibt.

## Start

```bash
go test -race ./...
cd apps/dashboard && npm install && npm run typecheck && cd ../..
make images load deploy-03
kubectl --context k3d-delivery-lab -n food-delivery port-forward service/dashboard 3000:3000
kubectl --context k3d-delivery-lab -n food-delivery port-forward service/control-api 8081:8080
```

Abnahme: Dashboard und API sind erreichbar, beide Healthchecks sind gruen, die Stadt bewegt sich ohne Teleportation und die DNS-Aufloesung im Cluster ist nachgewiesen.
