# Betrieb auf homeserver

Die Anwendung ist unter http://homeserver:7746 bzw.
http://192.168.254.100:7746 erreichbar. Ein eigenes Konto über die Registrierung
anlegen. Die vorhandenen Abnahmekonten enthalten nur Testdaten in eigenen Gruppen.

Auf dem Server mit `ssh server@homeserver` anmelden und wechseln:

```sh
cd /home/server/homebox-custom-src/deploy
```

Start, Stop und Status:

```sh
docker compose start
docker compose stop
docker compose ps
```

Ein Stop entfernt keine Daten. Datenbank und Uploads liegen gemeinsam in `data/`.
Die private `.env` enthält Build-Commit, Port und ein generiertes Geheimnis.

Vor einem Update ein Backup erstellen:

```sh
sh backup.sh
```

Das Skript stoppt kurz ausschließlich diese Instanz, sichert `data`, `.env` und
Compose nach `backups/` und startet die Instanz wieder. Backups privat aufbewahren
und zusätzlich auf einen anderen Datenträger kopieren.

Update: Im Quellcode-Verzeichnis den gewünschten geprüften Commit holen und
auschecken. `.env` und `data/` behalten. `CUSTOM_COMMIT` in `.env` auf diesen Commit
setzen; anschließend:

```sh
docker compose build
docker compose up -d
docker compose ps
```

Build-Commit in der Web-Fußzeile kontrollieren. Neue Upstream-Versionen zunächst
in den eigenen Branch übernehmen und die Tests aus `ACCEPTANCE.md` wiederholen.

Wiederherstellung: Instanz stoppen, das aktuelle `data/` in ein datiertes
Sicherungsverzeichnis verschieben und das gewählte Backup in ein leeres
Deployment-Verzeichnis entpacken. Passenden Quellcode-Commit und Image bereitstellen,
danach Compose starten. Daten und Uploads prüfen, bevor die vorherige Ablage
entfernt wird. Niemals in eine laufende Datenbank entpacken. Eine Wiederherstellung
wurde tatsächlich in einer separaten temporären Instanz geprüft.

Dockerfile: `Dockerfile.custom`; Konfiguration: `deploy/compose.yaml`.
Technisches Verhalten der Operationen und Fehlerszenarien: `CUSTOM.md`.
Testergebnisse und Einschränkungen: `ACCEPTANCE.md`.
