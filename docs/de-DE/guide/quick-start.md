# In 5 Minuten loslegen

Wenn du nur den kürzesten Weg vom Download zum Spielen willst, folge zuerst dieser Seite. Wähle in einem beliebigen Tab **Windows** oder **Android**: Die ganze Anleitung folgt deiner Wahl.

## 1. LeviLauncher installieren

=== "Windows"

    - Lade das LeviLauncher Installationspaket von der [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases)-Seite oder vom [Lanzou Cloud Mirror](https://levimc.lanzoue.com/b016ke39hc) herunter (Passwort für Lanzou: `levi`).
    - Führe das Installationsprogramm unter Windows aus.
    - Falls während der Installation Abhängigkeiten wie WebView2 angefordert werden, erlaube bitte deren Installation.

=== "Android"

    - Lade die neueste APK von [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) herunter.
    - Erlaube die Installation von Apps über den verwendeten Browser oder Dateimanager und installiere die APK.
    - Öffne LeviLauncher und akzeptiere die Launcher-Vereinbarung und die angefragten Berechtigungen.

## 2. Minecraft vorbereiten

=== "Windows"

    - Melde dich mit einem Microsoft-Konto an, das über **Minecraft Bedrock Edition** verfügt.
    - Stelle sicher, dass das Spiel mindestens einmal über den Microsoft Store auf diesem Computer installiert wurde.
    - Vergewissere dich, dass **Gaming Services** verfügbar ist (für GDK-Versionen erforderlich).

    ::: warning Rechtmäßige Kopie erforderlich
    LeviLauncher verwaltet Minecraft Bedrock (GDK / UWP) und ersetzt nicht die Spielelizenz.
    :::

=== "Android"

    - Installiere **Minecraft Bedrock Edition** 1.21.80 oder neuer aus **Google Play** auf diesem Gerät.
    - LeviLauncher nutzt diese offizielle Kopie als vertrauenswürdige Spielquelle; sie ersetzt nicht die Spiellizenz.

## 3. Eine Version wählen

=== "Windows"

    - Starte LeviLauncher.
    - Gehe über die Seitenleiste zu **Herunterladen**.
    - Wähle, ob du einen **Release**- oder einen **Vorschau**-Build möchtest, und ein **GDK**- oder **UWP**-Paket.
    - **Release** und **Vorschau** bezeichnen hier den normalen Release-Build und den Preview-Build von Minecraft.
    - Wähle eine Version aus und beginne die Installation.

=== "Android"

    - Öffne die Versions- (Instanz-)Ansicht.
    - Wähle die Minecraft-Version, die du starten willst, oder importiere eine offizielle Minecraft-APK, um eine andere Version im Launcher zu starten.

## 4. Entscheide, ob Isolierung verwendet werden soll

=== "Windows"

    Isolierung speichert die Spieldaten dieser Version in einem separaten Arbeitsverzeichnis, statt sie mit der Standard-AppData-Umgebung zu vermischen. Sie ist für GDK-Versionen und für UWP-x64-Versionen ab 1.19.70.2 verfügbar.

    Verwende Isolierung in folgenden Fällen:

    - Du testest Mods und möchtest die Hauptumgebung nicht beeinträchtigen
    - Du möchtest Minecraft Release und Preview getrennt halten
    - Du möchtest Welten und Ressourcenpakete für verschiedene Zwecke getrennt verwalten

=== "Android"

    Aktiviere die Versionsisolierung, wenn du eine importierte Version startest, deren Welten, Pakete und Einstellungen von der aus Google Play installierten Version getrennt bleiben sollen. Importierte Versionen brauchen sie oft zum Starten.

## 5. Starte die installierte Version

=== "Windows"

    - Gehe zurück zur Seite **Starten**.
    - Wähle die gerade installierte Version aus.
    - Klicke auf **Starten**.

    Falls LeviLauncher über fehlende GameInput- oder Gaming Services-Komponenten berichtet, folge bitte zunächst den Anweisungen zur Abhängigkeitsinstallation.

=== "Android"

    - Kehre zur Startansicht zurück und tippe auf **Starten**.
    - Schlägt der Start fehl, öffne das Startprotokoll in der Fehleransicht und prüfe den Grund, bevor du Einstellungen änderst.

## 6. Weiterführende Kapitel

- Umgebung einrichten: [Systemanforderungen und Installation](./requirements-installation)
- Erster Start: [Erster Start](./first-launch)
- Versionsübersicht: [Versionsverwaltung](./version-management)
- Inhalte verwalten: [Inhaltsverwaltung](./content-management)
- Mods und Integrationen: [Mods und Integration](./mods-integrations)
