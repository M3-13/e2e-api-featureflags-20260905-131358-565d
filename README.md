# Feature-Flag-Service

Ein Feature-Flag-Service als REST-API in Go, ausschließlich mit `net/http` aus
der Standardbibliothek. Flags werden in einem thread-sicheren In-Memory-Store
gehalten und über CRUD-Endpunkte verwaltet. Ein Evaluate-Endpunkt liefert pro
Nutzer eine deterministische Ja/Nein-Entscheidung auf Basis eines stabilen
Hashs aus `key` und `user` gegen `rollout_percent`. Dazu kommen Eingabevalidierung
mit sauberen Statuscodes und JSON-Fehlerobjekten sowie Zugriffs-Logging als
Middleware.

## Tech Stack

- **Sprache**: Go (>= 1.22)
- **Framework**: `net/http` (Standardbibliothek, keine externen Dependencies)
- **Tests**: `testing` + `net/http/httptest`
- **Synchronisierung**: `sync.Mutex`

## Installation

Es sind keine externen Abhängigkeiten nötig. Voraussetzung ist eine
Go-Installation (>= 1.22).

## Start (Dev)

```sh
go run .
```

Der Service lauscht standardmäßig auf `127.0.0.1:8080`. Der Port kann über die
Umgebungsvariable `PORT` geändert werden:

```sh
PORT=9000 go run .
```

## Env-Variablen

| Variable | Default | Beschreibung |
| --- | --- | --- |
| `PORT` | `8080` | TCP-Port, auf dem der Service lauscht (Host ist fest `127.0.0.1`). |

## Endpunkte

| Methode | Pfad | Beschreibung |
| --- | --- | --- |
| `GET` | `/healthz` | Health-Check, liefert `200` mit leerem Body. |
| `POST` | `/flags` | Legt ein Flag an. Body: `{"key": string, "enabled": bool, "description": string, "rollout_percent": int}` → `201` Flag \| `400`. |
| `GET` | `/flags` | Listet alle Flags → `200` `[Flag]`. |
| `GET` | `/flags/{key}` | Holt ein Flag → `200` Flag \| `404`. |
| `PUT` | `/flags/{key}` | Aktualisiert ein Flag (nur vorhandene Felder) → `200` Flag \| `400` \| `404`. |
| `DELETE` | `/flags/{key}` | Löscht ein Flag → `204` \| `404`. |
| `GET` | `/flags/{key}/evaluate?user={id}` | Deterministische Rollout-Entscheidung → `200` `{"key":"…","enabled":bool}` \| `400` \| `404`. |

### Flag-Schema

```json
{
  "key": "my-feature",
  "enabled": true,
  "description": "opt-in feature",
  "rollout_percent": 50
}
```

Fehlerantworten (Status >= 400) haben ausschließlich die Form
`{"error": "…"}`.

## Features

- CRUD-Verwaltung von Feature-Flags in einem thread-sicheren In-Memory-Store.
- Deterministischer Evaluate-Endpunkt (FNV-64a über `key` + `\x00` + `user`).
- Eingabevalidierung mit sauberen Statuscodes und JSON-Fehlerobjekten.
- Zugriffs-Logging als Middleware (Methode, Pfad ohne Query, Statuscode, Dauer).
