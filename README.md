# 🌌 Aurora Engine

**Challenge Task FS 2026 – Distributed Systems**

## Die Vision

Die **Aurora Engine** ist eine hochverfügbare, verteilte Media-Processing-Pipeline. Das Projekt fokussiert sich auf die asynchrone und ausfallsichere Verarbeitung von ressourcenintensiven Aufgaben (Video-Uploads und Processing).

Das Ziel: Ein System, das nicht nur unter Last performt, sondern bei dem ein Node-Ausfall mitten in der Verarbeitung nahtlos vom verbleibenden Node übernommen wird.

## Architektur

```
                            ┌──────────┐
                            │  Client  │
                            └────┬─────┘
                                 │ :80
                           ┌─────▼──────┐
                           │  Traefik   │
                           │ Round-Robin │
                           └──┬──────┬──┘
              ┌───────────────┘      └───────────────┐
              ▼                                      ▼
       ┌─────────────┐                        ┌─────────────┐
       │    api-1    │                        │    api-2    │
       │             │                        │             │
       │ HTTP Server │                        │ HTTP Server │
       │ Worker Loop │◄───── XCLAIM ────────► │ Worker Loop │
       │ PEL Claimer │      (Failover)        │ PEL Claimer │
       └──────┬──────┘                        └──────┬──────┘
              │                                      │
    ┌─────────┼──────────────────────────────────────┼─────────┐
    │         │            aurora-internal            │         │
    │         ▼                  ▼                    ▼         │
    │   ┌──────────┐      ┌──────────┐        ┌──────────┐    │
    │   │ Postgres │      │  Redis   │        │  RustFS  │    │
    │   │  Users   │      │ Streams  │        │   (S3)   │    │
    │   │  Videos  │      │  CG/PEL  │        │ Uploads  │    │
    │   │   Jobs   │      │          │        │ Processed│    │
    │   └──────────┘      └──────────┘        └──────────┘    │
    └─────────────────────────────────────────────────────────┘
```

## Tech Stack

* **Backend:** Go – 2 identische Instanzen für Hochverfügbarkeit
* **Gateway:** Traefik v3 – verteilt Requests gleichmässig und erkennt ausgefallene Container
* **Database:** PostgreSQL 16 – User, Video-Metadaten und Processing Jobs
* **Message Broker:** Redis Streams – Consumer Groups (CG) mit Pending Entries List (PEL) für Failover
* **Storage:** RustFS – S3-kompatibler Objektspeicher, angesprochen via minio-go Client
* **Frontend:** Plain HTML
* **Auth:** JWT Bearer Tokens

## Das Failover-Szenario

1. Videos werden hochgeladen und als Events in den Redis Stream gepublished (`XADD`).
2. Beide Instanzen lesen als Consumer Group – jede Nachricht geht an genau einen Worker.
3. **Failover:** Eine Instanz wird während der Verarbeitung hart beendet (`docker stop`).
4. **Recovery:** Die überlebende Instanz erkennt über `XPENDING` die unbestätigten Nachrichten, claimt sie via `XCLAIM` und verarbeitet sie – kein Datenverlust.

## Phasen

| Phase | Thema | Status |
|-------|-------|--------|
| 1 | Infrastruktur (Docker Compose, Traefik, Postgres, Redis, RustFS) | Done |
| 2 | Go-App Grundgerüst (Config, DI, Health-Endpunkte, Graceful Shutdown) | Done |
| 3 | JWT Auth, Video-Metadaten-CRUD, Pagination/Filter, Unit-Tests | Done |
| 4 | Streaming Upload nach RustFS (multipart/form-data), Metadaten in Postgres | Done |
| 5 | Redis Streams Publisher, Consumer-Group Worker, PEL-Claiming/Failover | Done |
| 6 | Processing-Logik (Jobs erstellen, Video-Status-Pipeline, simulierte Arbeit) | Offen |
| 7 | Video-Transcoding-Logik (ffmpeg-Transcoding, Skalierung, Output nach RustFS) | Offen |
| 8 | Failover-Demo unter Last | Offen |
| 9 | Web UI | Offen |

## API

**Auth**
- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /api/users/me` (Bearer Token erforderlich)

**Videos**
- `POST /api/upload` – Streaming Multipart-Upload nach RustFS
- `POST /api/videos` – Metadaten manuell anlegen
- `GET /api/videos` – Liste mit Pagination (`page`, `limit`), Filter (`status`, `q`)
- `GET /api/videos/:id`
- `PUT /api/videos/:id`
- `DELETE /api/videos/:id`

**System**
- `GET /api/health`
- `GET /api/health/deps` – Postgres, Redis, RustFS Status

Alle Fehlerantworten folgen einem konsistenten JSON-Format:
```json
{ "error": { "code": "bad_request", "message": "..." } }
```

## Quickstart

```bash
docker compose up --build
```

Traefik Dashboard: `http://localhost:8081`
API: `http://localhost/api/health`
RustFS Console: `http://localhost:9001`
