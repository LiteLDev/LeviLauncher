# Versionsverwaltung

LeviLauncher behandelt jede installierte Umgebung als eine eigenständig verwaltbare Version. In der App heißen installierte Versionen Instanzen und werden auf der Seite **Instanzen** aufgelistet.

## Was du tun kannst

=== "Windows"

    - Installieren von Release- oder Preview-Versionen
    - Festlegen eines leicht erkennbaren Namens für die Version
    - Schnelles Starten einer bestimmten Version
    - Löschen nicht mehr benötigter Versionen
    - Öffnen versionsspezifischer Einstellungen
    - Erstellen einer Desktop-Verknüpfung für die Version

=== "Android"

    - offizielle Minecraft-APKs importieren und im Launcher starten, ohne jede Version im System zu installieren
    - mehrere Versionen und ihre Daten getrennt halten
    - die Version wählen, die die Startansicht startet

## Der Unterschied zwischen Release und Preview <Badge platform="windows" />

| Typ | Geeignet für | Beschreibung |
| --- | --- | --- |
| Release | Stabiles, tägliches Spielen | Geeignet für die meisten Spieler |
| Preview | Vorab-Testen neuer Inhalte | Anfälliger für Änderungen bei Welt-, Paket- oder Mod-Kompatibilität |

## Was ist Isolierung?

=== "Windows"

    Isolierung ist für GDK-Versionen und für UWP-x64-Versionen ab 1.19.70.2 verfügbar. Wenn die Isolierung aktiviert ist, speichert LeviLauncher die Spieldaten dieser Version in einem eigenen Arbeitsbereich, anstatt sie mit dem Standardspeicherort zu teilen.

    Isolierung ist für folgende Anforderungen geeignet:

    - Unterschiedliche Inhaltskombinationen für verschiedene Versionen
    - Sichereres Testen von Mods oder Minecraft Preview
    - Verringerung des Risikos, dass eine Umgebung eine andere beeinträchtigt

=== "Android"

    Aktiviere die Isolierung für eine importierte Version, wenn ihre Welten, Pakete und Einstellungen von der aus Google Play installierten Version getrennt bleiben sollen. Importierte Versionen brauchen die Isolierung oft zum Starten.

## Häufige Versionsoperationen <Badge platform="windows" />

### Neue Version installieren

Führe die Installation auf der Seite **Herunterladen** durch und kehre anschließend zu **Starten** zurück, um zu starten.

### Version umbenennen

Lege unter **Instanzen** oder **Instanz-Einstellungen** einen aussagekräftigeren Namen für die Umgebung fest, z. B. `Release Survival` oder `Preview Test`.

### Version löschen

Entferne nicht mehr benötigte Versionen, insbesondere abgeschlossene Minecraft Preview-Umgebungen.

::: warning Vor dem Löschen prüfen
Das Löschen einer Version kann die zugehörigen Inhalte in dieser verwalteten Umgebung entfernen. Sichere vorher wichtige Welten.
:::

### Desktop-Verknüpfung erstellen

Verwende **Desktop-Verknüpfung erstellen** im Menü neben der Schaltfläche **Starten**, um für eine bestimmte Version eine Verknüpfung zu erstellen, mit der du sie direkt starten kannst.

## Empfohlene Versionsstrategie <Badge platform="windows" />

- Eine isolierte Release-Version für das tägliche Spielen
- Eine isolierte Preview-Version nur bei Bedarf
- Eine temporäre Testversion für Mods oder zur Fehlerbehebung

## Verwandte Lektüre

- [Download und Spiegelserver](./downloads-mirrors)
- [Welt-Tools](./world-tools)
- [Updates und Fehlerbehebung](./update-troubleshooting)
