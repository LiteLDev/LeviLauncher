# Anforderungen & Installation

Diese Seite erklärt, was LeviLauncher braucht, bevor es Minecraft Bedrock Edition unter Windows und auf Android installieren und starten kann.

UWP-Versionen erfordern den Windows-Entwicklermodus sowie die vom Paket deklarierten UWP-Framework-Abhängigkeiten (z. B. Microsoft.VCLibs). Die unten genannten Anforderungen an Gaming Services und GameInput gelten für GDK. Die Instanzisolierung funktioniert für GDK und für UWP-x64-Pakete ab Version 1.19.70.2; nicht isolierte UWP-Instanzen teilen sich den `LocalState`-Inhalt der Windows-App. UWP-x64-Instanzen unterstützen außerdem die Konsole und kompatible `preload-native`-DLL-Mods, die automatische Installation von LeviLamina ist jedoch nur für unterstützte GDK-Versionen verfügbar.

## Systemanforderungen

=== "Windows"

    | Artikel | Anforderung |
    | :--- | :--- |
    | Betriebssystem | Windows 10 oder Windows 11 |
    | Spielversion | Minecraft Bedrock Edition (GDK / UWP) |
    | Lizenz | Original-Lizenz, gebunden an ein Microsoft-Konto |
    | Netzwerk | Zum Herunterladen von Versionen, Abrufen von Metadaten, Messen der Spiegelgeschwindigkeit und Prüfen auf Updates |

=== "Android"

    | Punkt | Anforderung |
    | --- | --- |
    | Betriebssystem | Android 9.0 (API 28) oder neuer |
    | Architektur | ARM64 (arm64-v8a) |
    | Arbeitsspeicher | 1 GB freier RAM (3 GB oder mehr empfohlen) |
    | Speicher | 2 GB frei für Minecraft und Spieldaten (5 GB oder mehr empfohlen) |
    | Spiel | Minecraft Bedrock Edition 1.21.80 oder neuer, aus Google Play installiert |
    | Lizenz | Eine rechtmäßige, in Google Play gekaufte Kopie von Minecraft |

## Erforderliche Windows-Komponenten <Badge platform="windows" />

Bevor du LeviLauncher zum ersten Mal startest oder installierst, kann es sein, dass dich der Launcher zur Installation fehlender Komponenten auffordert.

- **Microsoft Gaming Services** (GDK)
- **Microsoft GameInput** (GDK)
- **WebView2 Runtime**
- **Microsoft Visual C++ Redistributable (x64)**, wird vom nativen Mod-Loader benötigt; LeviLauncher prüft, ob es vorhanden ist, und bietet die Installation an

Ob diese fehlen, hängt vom aktuellen Zustand deiner Windows-Umgebung ab.

### Eigene WebView2 Fixed Version Runtime verwenden

Wenn die Evergreen-WebView2-Runtime nicht installiert ist, du aber selbst eine WebView2 Fixed Version Runtime verwaltest, starte LeviLauncher auf eine der folgenden Arten.

Gib das Verzeichnis in der Befehlszeile an:

```powershell
LeviLauncher.exe --webview2-runtime-dir "D:\WebView2\FixedRuntime"
```

Alternativ erstellst du `webview2.json` neben `LeviLauncher.exe`:

```json
{
  "browserExecutableFolder": "D:\\WebView2\\FixedRuntime"
}
```

Relative Pfade werden vom Verzeichnis aus aufgelöst, das `webview2.json` enthält. Das gewählte Verzeichnis muss `msedgewebview2.exe` direkt enthalten. Die Befehlszeilenoption hat Vorrang vor der Konfigurationsdatei. Ist keines von beiden konfiguriert, verwendet LeviLauncher weiterhin die systemweit installierte WebView2-Runtime.

## Vor der Installation einer Version

=== "Windows"

    Bitte schließe diese Checkliste ab:

    1. Installiere mindestens einmal Minecraft Bedrock über den Microsoft Store.
    2. Falls der Store-Status abnormal ist, starte das Spiel einmal, um die Installation zu bestätigen.
    3. Bevor du Versionen mit LeviLauncher installierst oder verwaltest, schließe das Spiel.

=== "Android"

    1. Installiere Minecraft Bedrock Edition aus Google Play auf dem Gerät.
    2. Aktualisiere es auf 1.21.80 oder neuer: Ältere Versionen lehnt der Launcher ab.
    3. Sorge für genug freien Speicher für die Versionen und Welten, die du behalten willst.

## Installation von LeviLauncher (Core)

=== "Windows"

    ### Lösung A: GitHub Releases Seite

    Geeignet für Nutzer, die das Installationspaket direkt von der offiziellen LeviLauncher Seite beziehen und gleichzeitig die Update-Logs einsehen möchten.

    1. Öffne die [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) Seite von LeviLauncher.
    2. Lade das Installationsprogramm herunter.
    3. Führe den Installationsassistenten aus und beende ihn.

    ### Lösung B: Lanzou Cloud Mirror

    Wenn GitHub in deiner Region langsam ist, ist dieser Eingang normalerweise bequemer.

    1. Öffne [Lanzou Cloud](https://levimc.lanzoue.com/b016ke39hc).
    2. Gib das Passwort `levi` ein.
    3. Lade die Datei herunter und führe das Installationsprogramm lokal aus.

=== "Android"

    1. Öffne die Seite [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) der Android-Version.
    2. Lade die neueste APK herunter.
    3. Erlaube die Installation über den Browser oder Dateimanager, mit dem du sie heruntergeladen hast.
    4. Installiere die APK und öffne sie (die App heißt „LeviLauncher“).
    5. Akzeptiere die Launcher-Vereinbarung und erteile die angefragten Berechtigungen.

## Installation der ersten verwalteten Version

=== "Windows"

    1. Öffne **Herunterladen** in LeviLauncher.
    2. Wähle **Release** oder **Vorschau** für den gewünschten Minecraft-Build sowie **GDK** oder **UWP** als Pakettyp.
    3. Wähle einen Versionseintrag aus.
    4. Entscheide, ob du die Isolierung aktivieren möchtest.
    5. Beginne die Installation und warte auf den Abschluss.

=== "Android"

    1. Öffne die Versions- (Instanz-)Ansicht.
    2. Wähle die aus Google Play installierte Version oder importiere eine offizielle Minecraft-APK.
    3. Aktiviere die Isolierung für eine importierte Version, deren Daten getrennt bleiben sollen.
    4. Kehre zur Startansicht zurück und tippe auf **Starten**.

## Empfohlene Installationsstrategie <Badge platform="windows" />

### Wann Version (Release) wählen?

- Wenn du eine stabilere Umgebung für den täglichen Spielbetrieb möchtest
- Wenn du an einer Langzeitwelt arbeitest
- Wenn du möchtest, dass Mods und Ressourcenpakete seltener Änderungen unterliegen

### Wann Preview (Vorschau) wählen?

- Wenn du zukünftige Funktionen vorab testen möchtest
- Wenn du Instabilität oder Kompatibilitätsänderungen akzeptieren kannst
- Wenn du die Minecraft Preview (Vorschau) Umgebung getrennt von deiner täglichen Spielumgebung halten möchtest

::: tip Empfohlene Vorgehensweise für die meisten Spieler
Erstelle zunächst eine **isolierte Release-Version**. Erstelle erst dann eine **Preview-Version**, wenn du explizit Inhalte testen möchtest.
:::

## Wenn die Installation nicht fortgesetzt werden kann

=== "Windows"

    Folgende Probleme können im Kapitel [Updates und Fehlerbehebung](./update-troubleshooting) weiter behandelt werden:

    - Fehlende Gaming Services
    - Fehlendes GameInput
    - Unvollständiger Store-Lizenz- oder Installationsstatus
    - Unbeschreibbarer Zielpfad
    - Download- oder Spiegel-Fehler

=== "Android"

    - Meldet der Launcher, dass Minecraft fehlt oder nicht unterstützt wird, installiere oder aktualisiere zuerst die Google-Play-Version.
    - Versionen vor 1.21.80 werden nicht unterstützt.
    - Bei Startfehlern siehe [Update & Fehlerbehebung](./update-troubleshooting).
