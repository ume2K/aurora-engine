# 🌌 Aurora Engine

**Challenge Task FS 2026 – Distributed Systems**

## Die Vision
Die **Aurora Engine** ist eine hochverfügbare, verteilte Media-Processing-Pipeline. Anstatt eines simplen CRUD-Systems (wie einem Standard-Blog) fokussiert sich dieses Projekt auf die asynchrone und ausfallsichere Verarbeitung von ressourcenintensiven Aufgaben (wie z.B. Video-Uploads und Transcoding). 

Das Ziel: Ein System, das nicht nur unter Last performt, sondern bei dem ein Node-Ausfall (Failover) mitten im Rechenprozess nahtlos vom restlichen Cluster abgefangen wird.

## Geplante Architektur & Tech Stack
Um die Komplexität auf die verteilte Logik zu lenken, wird die Infrastruktur modern, aber schlank gehalten:

* **Backend:** Go. Exakt 2 Instanzen für den Failover-Beweis.
* **Load Balancer / Gateway:** Traefik (verteilt Traffic und erkennt tote Container).
* **Database:** PostgreSQL. Persistenter Speicher für User und Video-Metadaten.
* **Message Broker:** Redis Streams mit Consumer Groups für failover-fähige Worker-Verarbeitung.
* **Storage:** RustFS (100% S3-kompatibler Objektspeicher für Uploads und verarbeitete Artefakte).
* **Frontend:** Plain HTML (Fokus auf Polling/Status-Updates und HLS-Playback).
* **Auth:** JWT-basierte Authentifizierung.

## Das Failover-Szenario (The Challenge)
Das Kernstück der Präsentation wird das Load Testing sein:
1. Das System wird unter Last gesetzt (`k6`).
2. Videos werden hochgeladen und von den Go-Workern verarbeitet.
3. **Failover:** Eine Go-Instanz wird während des Encodings hart beendet (`docker stop`).
4. **Recovery:** Die überlebende Instanz erkennt abgebrochene Jobs über die Redis Pending Entries List (PEL), claimt sie und beendet die Arbeit ohne Datenverlust für den Endnutzer.

## Status
* [x] Projektidee und Architektur-Entscheidung
* [x] Infrastruktur Setup (`docker-compose.yml`)
* [x] GoCore Integration
* [ ] JWT Setup
* [ ] ...
