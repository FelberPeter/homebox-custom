# Abnahme des angepassten Homebox-Forks

Prüfdatum: 05.10.2026. Ausgangsbasis: `sysadminsmedia/homebox` **v0.26.2**, Commit
`e01dd737238a3fa7e1a6454b37de6c6fc88c86e4`. Git-Upstream ist eingerichtet.
Fork: https://github.com/FelberPeter/homebox-custom, Branch `custom/mobile-inventory`.

## Bereitstellung

- URL: http://homeserver:7746, alternativ http://192.168.254.100:7746.
- Server: Linux x86_64, Docker 29.8.2, Compose 5.6.0.
- Compose-Projekt und Container: `homebox-custom`.
- Quellcode: `/home/server/homebox-custom-src`.
- Persistente Daten und Uploads: `/home/server/homebox-custom-src/deploy/data`.
- Build-Commit: `a74cbcd31a3854605b73ef31ba5dffecc925cc22`.
- Build-Version: `v0.26.2+custom`. Der vollständige Build-Commit steht in der
  Fußzeile, `/api/v1/status`, dem Image-Tag und den OCI-Labels.
- Eigene Testdatenbank; keine echten Bestandsdaten importiert. Andere Container
  und bestehende Dienste wurden nicht verändert. Kein öffentlicher Proxy oder
  Router-Port wurde eingerichtet.
- Registrierung ist verfügbar. Für die eigene Nutzung ein eigenes Konto anlegen;
  die Abnahmekonten enthalten ausschließlich Testdaten in getrennten Gruppen.
  Testzugänge und `.env` liegen privat auf dem Server und nicht in Git.

## Ergebnisse

| Nr. | Abnahmekriterium | Ergebnis und Nachweis |
| --- | --- | --- |
| 1 | Zwölf nummerierte Fächer mit korrektem Parent | Bestanden: Repository-Test und reale HTTP-Integration |
| 2 | 3×4-Raster mit zwölf Namen | Bestanden: Repository, HTTP und Browser; A1–C4 |
| 3 | Startwerte, Nullen und Muster | Bestanden: Nummern und Raster; ungültige Muster/Zahlen abgewiesen |
| 4 | Später weitere Fächer ergänzen | Bestanden: HTTP und Browser, vorhandene Fächer erhalten |
| 5 | Leere Fächer sichtbar | Bestanden: direkte leere Unter-Lagerorte als Kacheln, natürliche Sortierung |
| 6 | Kopiertiefe 0, 1, 2 und alle | Bestanden: Repository und reale HTTP-Integration |
| 7 | Items ein/aus und Mengen | Bestanden: Mengen 7 und 3; Item-Unterstrukturen erhalten |
| 8 | Neue IDs beim Kopieren, stabile IDs beim Verschieben | Bestanden: neue UUIDs und Asset-IDs; Browser-Move behält URL/ID |
| 9 | Namenskonflikte ohne Überschreiben | Bestanden: Vorschau, Blockierung ohne Zustimmung, bewusstes Fortfahren |
| 10 | Inhalte beim Verschieben, keine Zyklen | Bestanden: HTTP und Repository, einschließlich regulärer Edit-API |
| 11 | Unabhängige Fotos und Anhänge | Bestanden: Dateien und Vorschaubilder nach Löschen des Originals und einer weiteren Kopie lesbar |
| 12 | Individuelle Daten gemäß Optionen | Bestanden: Seriennummer/Kauf/Garantie aus und ein; Notizen, Felder und Tags erhalten |
| 13 | Fehler und wiederholte Requests | Bestanden: Rollback bei Dateifehler, dauerhafte Wiederholungserkennung, parallele Doppelübermittlung, geänderte Nutzlast abgewiesen |
| 14 | Anmeldung, Rechte und Gruppentrennung | Bestanden: vorhandene Middleware erhalten; nicht angemeldete und gruppenfremde Aktionen abgewiesen |
| 15 | Reduzierte Oberfläche | Bestanden: keine Preis-/Versicherungsfelder oder Wartungsnavigation; monetärer Berichtseinstieg entfernt |
| 16 | Handybreite | Bestanden in Chromium bei 360×800 CSS-Pixeln: Erzeugung, Vorschau, Kopieren, Verschieben und reguläre Erfassung; kein horizontales Scrollen |
| 17 | Neustart erhält Daten und Uploads | Bestanden: echter Compose-Neustart; Datensätze, Dateien und Wiederholungskennungen erhalten |

Zusätzlich wurde ein echtes Backup mit gestopptem Container erstellt und in einer
separaten temporären Instanz auf `127.0.0.1:7747` wiederhergestellt. Die Prüfung
bestätigte erhaltene Daten, Uploads und Wiederholungskennungen. Diese Prüf-Instanz
wurde anschließend gestoppt und entfernt. Die reguläre Instanz läuft weiter.
Das Backup besitzt Dateirechte `0600`.

## Ausgeführte Prüfungen

- Vollständige Go-Suite: `go test ./...` erfolgreich auf dem Server.
- Neue Repository-Tests einschließlich Thumbnail-Eigentümerschaft und Rollback
  erfolgreich; vorhandene Backend-Tests bleiben erfolgreich.
- Reproduzierbarer Linux-Docker-Produktionsbuild erfolgreich.
- ESLint für geänderte Vue-/TypeScript-Dateien erfolgreich.
- Vorhandene fokussierte Vitest-Suite: 3 Dateien, 19 Tests erfolgreich.
- Reale Integration: `python3 deploy/verify.py`, anschließend echter Neustart und
  `python3 deploy/verify.py --persistence` erfolgreich. Der Test wartet vor Beginn
  auf den lesenden Health-Endpunkt; mutierende Tests werden nicht automatisch
  wiederholt.
- Browser: deutsche Oberfläche, sichtbare Lagerpfade und Platzhalter, Vorschau,
  Erzeugen, Kopieren eines vollständigen Baums, Verschieben mit gleicher ID sowie
  Erfassung eines normalen Gegenstands mit Menge 5. Screenshots liegen lokal unter
  `output/playwright/` und werden nicht mit Testzugängen veröffentlicht.

## Bekannte Grenzen

- Kein physisches Android-Gerät war verfügbar. Die Prüfung erfolgte mit Chromium
  bei Handybreite; echte Android-Kameraaufnahme wurde nicht geprüft.
- Direkter LAN-Zugriff erfolgt über HTTP. Datei-/Fotoupload bleibt erhalten;
  Kamera-APIs, AR-Scanner und Service Worker benötigen vertrauenswürdiges HTTPS.
- Der eigenständige TypeScript-Check scheitert bereits im unveränderten Upstream
  mit 198 Diagnosen. Der angepasste Stand hat keine neuen Diagnose-Signaturen;
  Produktionsbuild, Lint und die ausgeführten Tests bestehen trotzdem. Die
  allgemeine TypeScript-Bereinigung gehört nicht zu diesem Fork.
- Ein bestehender Nuxt-Manifest-Matcher-Fehler erscheint in der Browserkonsole;
  die geprüften Abläufe funktionieren. Einige allgemeine Upstream-Beschriftungen
  sind weiterhin englisch, die neuen Abläufe und die reguläre Erfassung deutsch.
- Die neuen Operationen unterstützen diese Installation mit einer App-Replik.
  Bei einem Prozessabbruch während der Dateikopie können unreferenzierte Dateien
  zurückbleiben; es wird keine teilweise Inventarstruktur committed. Details und
  Wiederholungsverhalten stehen in `CUSTOM.md`.
- Externe URL-Anhänge behalten ihre URL. Die Unabhängigkeit gespeicherter Dateien
  gilt für lokal hochgeladene Anhänge und deren Vorschaubilder.
