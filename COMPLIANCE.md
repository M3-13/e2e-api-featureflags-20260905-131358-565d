VERDICT: CHANGES_REQUESTED

## Prüfrahmen

Geprüft wurde der gemergte Stand des Produkts: reine Go-Backend-REST-API (`net/http`, In-Memory-Store, keine öffentliche UI, keine KI-Funktion). Maßgeblich sind daher DSGVO und EU Cyber Resilience Act (CRA). AI Act, Impressums-/Cookie-/Barrierefreiheitspflichten sind mangels UI bzw. KI nicht anwendbar.

---

## 1. DSGVO

### 1.1 `user` wird im Klartext als Query-Parameter übermittelt
**Schweregrad: medium**

**Sachverhalt:**  
In `evaluate.go` wird der Nutzerwert über `r.URL.Query().Get("user")` gelesen. Die Spec (`AC-06`) verlangt explizit `GET /flags/{key}/evaluate?user={id}`. Der Wert ist potenziell personenbezogen (Nutzer-ID). Der Service selbst loggt den Query-String nicht, aber Query-Strings können in vorgelagerten Proxys, Access-Logs oder Browser-Historien landen. Das ist ein vermeidbares Übermittlungsrisiko.

**Konkrete Abhilfe:**  
- In `README.md` oder einer neuen Datei `SECURITY.md` verbindlich dokumentieren:
  - Der Evaluate-Endpunkt darf ausschließlich über TLS erreichbar sein.
  - Vorgelagerte Reverse-Proxies/Load-Balancer dürfen den Query-String nicht in Logs speichern.
- Optional und abwärtskompatibel: In `evaluate.go` zusätzlich einen Header `X-User` akzeptieren, der Vorrang vor dem Query-Parameter hat. So bleibt die Spec-konforme GET-Variante erhalten, aber datenschutzfreundlichere Integration wird möglich.

### 1.2 `key` wird im Pfad geloggt, ohne dass der Zeichenraum eingeschränkt ist
**Schweregrad: medium**

**Sachverhalt:**  
Die Logging-Middleware in `middleware.go` protokolliert `r.URL.Path`. Der Pfad enthält bei `/flags/{key}/...` den vom API-Nutzer frei gewählten `key`. Da `key` derzeit nur auf Länge (max. 200), aber nicht auf zulässige Zeichen begrenzt wird, kann ein API-Nutzer personenbezogene Daten (z. B. E-Mail-Adresse, Klarname) als `key` verwenden. Diese landen dann im Log.

**Konkrete Abhilfe:**  
In `store.go` die Funktion `validateKey` um eine Zeichenbeschränkung ergänzen, z. B.:

```go
import "regexp"

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func validateKey(key string) error {
    if key == "" {
        return ErrEmptyKey
    }
    if len(key) > maxKeyLength {
        return ErrKeyTooLong
    }
    if !keyPattern.MatchString(key) {
        return ErrKeyInvalidChars
    }
    return nil
}
```

Dazu einen neuen Fehler `ErrKeyInvalidChars` definieren und in `handleCreateFlag` (`handlers.go`) mit einer 400er-Fehlermeldung wie `"key must contain only letters, digits, dot, underscore and hyphen"` behandeln. Dadurch wird ausgeschlossen, dass PII als Key in Pfad und Logs gelangt.

### 1.3 `description` ist ein unbegrenztes Freitextfeld
**Schweregrad: low**

**Sachverhalt:**  
`Flag.Description` in `flags.go` ist ein unbegrenzter String. Der API-Nutzer könnte dort personenbezogene Daten eingeben, die im In-Memory-Store gespeichert und über `GET /flags` bzw. `GET /flags/{key}` ausgeliefert werden.

**Konkrete Abhilfe:**  
- In `README.md` dokumentieren, dass Flags keine personenbezogenen Daten enthalten dürfen.
- Optional in `handleCreateFlag`/`handleUpdateFlag` (`handlers.go`) eine maximale Länge für `description` festlegen (z. B. 2000 Zeichen) und bei Überschreitung mit `400` + `{"error":"description too long"}` antworten. Dies dient der Datenminimierung, ohne legitime Beschreibungen praktisch einzuschränken.

### 1.4 Keine Datenschutz-/Auftragsverarbeitungs-Dokumentation
**Schweregrad: low**

**Sachverhalt:**  
Der Service verarbeitet `user` transient und speichert ihn nicht. Eine Dokumentation der Verarbeitung, der Rechtsgrundlage bzw. der Auftragsverarbeitung ist im sichtbaren Bestand nicht vorhanden. Da das Produkt als Backend an Betreiber ausgeliefert wird, fehlt eine klare datenschutzrechtliche Einordnung.

**Konkrete Abhilfe:**  
In `README.md` einen Abschnitt „Data Processing / DSGVO“ ergänzen:
- Verarbeitet werden: `key`, `enabled`, `description`, `rollout_percent`, `user` (nur transient für Hash).
- `user` wird nicht gespeichert, nicht geloggt, nicht zurückgegeben.
- Logs enthalten nur Methode, Pfad (ohne Query), Status, Dauer.
- Betreiber müssen eine Rechtsgrundlage für die Verarbeitung von Nutzer-IDs sicherstellen und ggf. einen Auftragsverarbeitungsvertrag mit dem Hersteller schließen.

### 1.5 Log-Aufbewahrung und Löschung nicht geregelt
**Schweregrad: low**

**Sachverhalt:**  
`middleware.go` schreibt über den Standard-Logger nach stdout. Eine Rotation, Aufbewahrungsfrist oder Löschregelung ist nicht sichtbar. Da keine `user`-Werte geloggt werden, ist das Risiko begrenzt, aber datenschutzrechtlich sollte die Log-Retention definiert sein.

**Konkrete Abhilfe:**  
In `README.md` oder `SECURITY.md` festhalten, dass stdout-Logs keine personenbezogenen Daten enthalten und die Retention/Rotation von der Deployment-Umgebung vorgegeben wird. Optional einen Hinweis ergänzen, dass Logs maximal z. B. 30 Tage aufbewahrt und dann gelöscht werden.

---

## 2. EU Cyber Resilience Act (CRA)

### 2.1 Keine Authentifizierung / Autorisierung
**Schweregrad: high**

**Sachverhalt:**  
`main.go` setzt `httpServer.Handler = server.routes()`. In `routes()` ist keine Authentifizierungs- oder Autorisierungs-Middleware vorhanden. Alle CRUD-Endpunkte (`POST /flags`, `PUT /flags/{key}`, `DELETE /flags/{key}`) sind für jeden erreichbar, der Netzwerkzugriff auf den Port hat. Das betrifft Security by design/default nach CRA Anhang I und ermöglicht unbefugte Änderungen oder Löschungen von Feature-Flags.

**Konkrete Abhilfe:**  
In einer neuen Datei `auth.go` eine Middleware ergänzen, die z. B. `Authorization: Bearer <token>` prüft. Token aus Env-Variable `AUTH_TOKEN` lesen, Default leer = keine Authentifizierung (nur für rein lokale Entwicklung). In `routes()` vor `Logging(mux)` schalten:

```go
func (s *Server) routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /healthz", s.handleHealthz)
    // ... restliche Routen
    return Logging(Auth(mux))
}
```

Fehlgeschlagene Authentifizierung liefert `401` im gleichen JSON-Fehlerformat `{"error":"unauthorized"}`. Tests in `auth_test.go` ergänzen. Alternativ: In `README.md` verbindlich dokumentieren, dass der Dienst ausschließlich hinter einem authentifizierenden API-Gateway betrieben werden darf.

### 2.2 Fehlende dokumentierte Sicherheitseigenschaften und SBOM
**Schweregrad: medium**

**Sachverhalt:**  
Der sichtbare Bestand enthält keine `SECURITY.md` oder einen Sicherheitsabschnitt im `README.md`. Es fehlen dokumentierte Sicherheitsannahmen, Bedrohungsmodell, Hinweise zu sicherem Deployment, Patch-/Update-Konzept und ein SBOM (Software Bill of Materials). `go.mod` enthält vermutlich nur die Standardbibliothek, aber die Abhängigkeiten sind nicht als SBOM sichtbar.

**Konkrete Abhilfe:**  
Neue Datei `SECURITY.md` anlegen mit:
- Sicherheitskonfiguration (Loopback, Timeouts, Body-Limit).
- Zugriffsschutz (siehe 2.1).
- TLS-Terminierung vor dem Dienst.
- SBOM: Ausgabe von `go list -m all` in `SECURITY.md` oder `sbom.txt` ablegen.
- Meldeprozess für Sicherheitslücken (z. B. Security-Kontakt).
- Update-/Patch-Prozess: Die Software wird über reguläre Releases/Deployments aktualisiert; es gibt keine eingebettete Update-Funktion.

### 2.3 Inkonsistente Key-Längenprüfung bei GET/PUT/DELETE
**Schweregrad: low**

**Sachverhalt:**  
`evaluate.go` und `store.Create` validieren die Key-Länge, aber `handleGetFlag`, `handleUpdateFlag` und `handleDeleteFlag` in `handlers.go` rufen den Store direkt mit `r.PathValue("key")` auf, ohne Längen-/Zeichenvalidierung. Zwar kann kein überlanger Key im Store existieren, aber die Endpunkte verarbeiten beliebig lange Strings aus dem Pfad.

**Konkrete Abhilfe:**  
Vor jedem Store-Zugriff in `handleGetFlag`, `handleUpdateFlag` und `handleDeleteFlag` eine gemeinsame Validierung aufrufen:

```go
key := r.PathValue("key")
if len(key) > maxKeyLength {
    writeError(w, http.StatusBadRequest, "key too long")
    return
}
```

Optional direkt `validateKey(key)` verwenden und `ErrKeyTooLong`/`ErrKeyInvalidChars` in 400er-Fehler übersetzen.

### 2.4 Fehlende Security-Header
**Schweregrad: low**

**Sachverhalt:**  
`respond.go` setzt lediglich `Content-Type: application/json`. Security by default legt für API-Antworten mindestens `X-Content-Type-Options: nosniff` nahe.

**Konkrete Abhilfe:**  
In `writeJSON` zusätzlich setzen:

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("Cache-Control", "no-store")
```

---

## 3. EU AI Act

Nicht anwendbar. Es sind im sichtbaren Stand keine KI-Funktionen bzw. KI-Systeme enthalten.

---

## 4. Pflichttexte & UI

Nicht anwendbar. Das Produkt ist eine reine Backend-API ohne öffentliche Web-Oberfläche; Impressum, Datenschutzerklärung im Browser, Cookie-Banner und Verbraucher-Widerrufsbelehrung sind nicht erforderlich.

---

## 5. Barrierefreiheit (WCAG/BITV/EAA)

Nicht anwendbar. Es gibt keine öffentliche UI; die API-Antworten sind JSON.

---

## Fazit

Der Service erfüllt wesentliche datenschutzrechtliche Vorgaben: `user` wird nicht gespeichert, nicht geloggt, der Store enthält kein `user`-Feld, Fehlerantworten geben keine internen Details preis, Timeouts und Body-Limit sind gesetzt, Standard-Host ist Loopback. Es bestehen jedoch behebbare Lücken, insbesondere die fehlende Authentifizierung sowie die fehlende Sicherheits-/Datenschutzdokumentation. Da kein personenbezogenes Datenleck im Code sichtbar ist und kein Verarbeitungsvorgang ohne Rechtsgrundlage implementiert ist, erfolgt keine Blockade, sondern eine Freigabe erst nach Umsetzung der geforderten Änderungen.