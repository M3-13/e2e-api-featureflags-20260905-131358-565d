VERDICT: CHANGES_REQUESTED

## Zusammenfassung

Der Feature-Flag-Service ist insgesamt solide umgesetzt: Secrets, SQL-/Command-Injection und bekannte Schwachstellen sind nicht sichtbar, die geforderten Timeouts und die Loopback-Bindung sind korrekt, Fehlerantworten sind weitgehend auf das Feld `error` reduziert, und sensible Query-Parameter werden nicht geloggt oder gespeichert. Es fehlen jedoch Zugriffsschutz und einige Eingabevalidierungen/Härtungen. Die Befunde sind überwiegend mittel/niedrig – keine kritische oder hohe Lücke.

## Befunde im Detail

### 1. Fehlende Authentifizierung/Autorisierung auf den CRUD-Endpunkten
- **Schweregrad:** medium
- **Betroffen:** `main.go` (Routing), `handlers.go` (`handleCreateFlag`, `handleUpdateFlag`, `handleDeleteFlag`)
- **Problem:** `POST /flags`, `PUT /flags/{key}` und `DELETE /flags/{key}` sind ohne jeden Zugriffsschutz erreichbar. Da der Server auf `127.0.0.1` lauscht, können lokale Prozesse uneingeschränkt Flags anlegen, ändern und löschen. Zusätzlich kann ein Browser-CSRF-Angriff über einen simplen `POST` mit `Content-Type: text/plain` zumindest neue Flags anlegen, weil der Content-Type nicht geprüft wird (Befund 2).
- **Fix:** Einen konfigurierbaren API-Key aus der Umgebungsvariable lesen und in einer Middleware prüfen, z. B.:
  - `FLAG_API_TOKEN` aus `os.Getenv` laden.
  - In einer neuen Middleware `Authorization: Bearer <token>` mit `subtle.ConstantTimeCompare` prüfen; fehlende/ungültige Tokens mit `401` beantworten.
  - Falls bewusst keine Auth gefordert ist, mindestens `Content-Type: application/json` erzwingen und ggf. den `Sec-Fetch-Site`-Header prüfen, um Cross-Origin-Schreibzugriffe zu reduzieren.

### 2. Content-Type wird nicht validiert
- **Schweregrad:** medium
- **Betroffen:** `handlers.go`, Funktion `decodeJSONBody`
- **Problem:** Der Request-Body wird unabhängig vom `Content-Type` als JSON dekodiert. Dadurch werden z. B. `text/plain` oder `application/x-www-form-urlencoded` akzeptiert. Das erleichtert CSRF über simple POSTs und verschleiert Client-Fehler.
- **Fix:** Zu Beginn von `decodeJSONBody` den Medientyp prüfen:
  ```go
  ct := r.Header.Get("Content-Type")
  if ct != "" {
      mt, _, err := mime.ParseMediaType(ct)
      if err != nil || mt != "application/json" {
          writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
          return errors.New("unsupported media type")
      }
  }
  ```
  `mime` aus der Standardbibliothek importieren. Das verhindert den einfachen `text/plain`-CSRF-Pfad und erzwingt semantisch korrekte Requests.

### 3. Restinhalt des Request-Bodys wird nicht konsumiert / nicht auf Trailing-Garbage geprüft
- **Schweregrad:** low
- **Betroffen:** `handlers.go`, Funktion `decodeJSONBody`
- **Problem:** `dec.Decode(v)` dekodiert nur das erste JSON-Dokument. Ein Body wie `{"key":"a"} <sehr große Restbytes>` kann dazu führen, dass der JSON-Anteil akzeptiert wird, während das `http.MaxBytesReader`-Limit für den Rest nie greift, weil der Rest nicht gelesen wird. Das schwächt die Wirksamkeit der Body-Limitierung (AC-13) ab und hinterlässt bei Keep-Alive ungelesene Daten.
- **Fix:** Nach erfolgreichem `Decode` den restlichen Body vollständig auslesen und dabei `MaxBytesError` korrekt auswerten:
  ```go
  if err := dec.Decode(v); err != nil { ... }

  if _, err := io.Copy(io.Discard, r.Body); err != nil {
      var maxErr *http.MaxBytesError
      if errors.As(err, &maxErr) {
          writeError(w, http.StatusBadRequest, "request body too large")
      } else {
          writeError(w, http.StatusBadRequest, "invalid JSON")
      }
      return err
  }
  ```
  Zusätzlich kann ein zweites `dec.Decode(&struct{}{})` genutzt werden, um Trailing-Garbage zu erkennen.

### 4. Key-Längenbegrenzung fehlt auf GET/PUT/DELETE
- **Schweregrad:** low
- **Betroffen:** `handlers.go` (`handleGetFlag`, `handleUpdateFlag`, `handleDeleteFlag`)
- **Problem:** `handleEvaluateFlag` prüft die Key-Länge, `handleCreateFlag` tut das über den Store. `GET /flags/{key}`, `PUT /flags/{key}` und `DELETE /flags/{key}` geben bei einem überlangen `{key}` jedoch nur `404` statt der laut AC-12 geforderten `400`-Ablehnung. Aktuell wird der überlange Key nicht gespeichert oder gehasht, daher kein hohes Risiko – aber AC-12 ist nicht vollständig erfüllt.
- **Fix:** In jedem der drei Handler vor dem Store-Zugriff validieren:
  ```go
  key := r.PathValue("key")
  if len(key) == 0 || len(key) > maxKeyLength {
      writeError(w, http.StatusBadRequest, "key must be at most 200 characters")
      return
  }
  ```
  Das sollte am besten in eine kleine Hilfsfunktion `validateKeyFromPath` zusammengefasst werden, um Duplizierung zu vermeiden.

### 5. Log-Injection über den URL-Pfad
- **Schweregrad:** low
- **Betroffen:** `middleware.go`, Funktion `Logging`
- **Problem:** `r.URL.Path` ist URL-dekodiert. Ein Pfad mit Steuerzeichen, z. B. `%0a`, kann Zeilenumbrüche in die Log-Ausgabe einfügen und lokale Log-Einträge verfälschen.
- **Fix:** Pfad in der Formatierung escapen, z. B. mit `%q`:
  ```go
  log.Printf("%s %q %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
  ```
  `%q` gibt den Pfad in Go-Quoting aus und escaped Steuerzeichen. Die bestehenden Logging-Tests bleiben erfüllt.

### 6. Default-Fehlerpfad gibt `err.Error()` aus
- **Schweregrad:** low
- **Betroffen:** `handlers.go`, `handleCreateFlag`
- **Problem:** Der `default`-Zweig reicht `err.Error()` an den Client weiter. Aktuell kommen dort nur die definierten Store-Fehler an; zukünftige interne Fehler könnten jedoch interne Details leaken und AC-11 verletzen.
- **Fix:** Kein `err.Error()` verwenden, sondern eine generische Fehlermeldung:
  ```go
  default:
      writeError(w, http.StatusBadRequest, "invalid flag")
  ```
  So bleibt die Fehlerantwort unabhängig von internen Fehlern ausschließlich das JSON-Objekt `{"error": ...}`.

## Positiv geprüfte Bereiche

- **Secrets:** Keine Hardcoded API-Keys, Passwörter oder Tokens sichtbar.
- **Injection/Inputs:** Kein SQL, Command oder Pfad-Traversal; in-memory Store, keine Shell-Aufrufe, keine unsichere Deserialisierung.
- **AuthN/AuthZ:** Der Dienst bindet nur an `127.0.0.1` (AC-15 erfüllt); fehlende Auth wurde oben als Befund aufgenommen.
- **Dependencies:** Alle sichtbaren Imports stammen aus der Go-Standardbibliothek; keine externen Pakete sichtbar. Es wurde kein Scanner-Output geliefert; das ist kein Beleg für Abwesenheit, aber es gibt keine Scanner-Funde zu interpretieren.
- **Konfiguration/Transport:** `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` sind gesetzt und in `main_test.go` abgesichert; Loopback-Bindung verhindert externen Netzwerkzugriff.
- **Datenschutz:** Logging protokolliert nur Methode, Pfad ohne Query, Status und Dauer; der `user`-Wert erscheint nicht im Log (AC-16). Der Evaluate-Endpunkt verändert den Store nicht (AC-17).

## Gesamteinschätzung

Das Produkt erfüllt die wesentlichen Sicherheitsanforderungen aus AC-11 bis AC-17 weitgehend. Die Beanstandungen betreffen vor allem fehlende Zugriffskontrolle, fehlende Content-Type-Prüfung sowie kleinere Validierungs-/Absicherungslücken. Daher sind Überarbeitungen angezeigt, aber kein sofortiger Versandstopp.