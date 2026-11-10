# Mods & Integrationen

LeviLauncher unterstützt Mod-bezogene Arbeitsabläufe, die über den Umfang eines „Basis-Launchers“ hinausgehen.

## Derzeit verfügbare Funktionen

=== "Windows"

    - Importieren von Mod-Paketen (`.mcmod`)
    - `.zip`-Pakete und einzelne `.dll`-Mods werden weiterhin akzeptiert: Wähle im Dateidialog **Alle Dateien (\*.\*)**
    - Aktivieren oder Deaktivieren installierter Mods
    - Löschen nicht mehr benötigter Mods
    - Nutzung von CurseForge-bezogenen Such- und Beschaffungsabläufen
    - Installieren von LIP-Paketen mit dem integrierten, [lip](https://github.com/LiteLDev/lip)-kompatiblen Paketmanager
    - Zusammenarbeit mit Loader-Workflows wie LeviLamina

    ::: warning Dies ist ein fortgeschrittener Bereich
    Die Kompatibilität von Mods, Loadern und Paket-Integrationen ändert sich oft schneller, insbesondere bei Minecraft Preview. Betrachte dies als fortgeschrittene Funktion und teste schrittweise.
    :::

=== "Android"

    - native Mods (`.so`-Bibliotheken) ins Spiel laden
    - gemoddete Setups in isolierten Versionen halten

    Du entwickelst native Mods? Siehe [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) in der LeviLamina-Dokumentation (auf Englisch).

## Empfohlener Mod-Arbeitsablauf

1. Beginne mit einer sauberen, isolierten Version.
2. Stelle zunächst sicher, dass die Version ohne Mods normal startet.
3. Füge jeweils nur einen Mod oder eine kleine Gruppe von Änderungen hinzu.
4. Teste den Start nach jeder Änderung erneut.
5. Bewahre für wichtige Welten immer einen Backup-Pfad auf.

## CurseForge <Badge platform="windows" />

Der Launcher bietet CurseForge-bezogene Such- und Paketabläufe, um dir das Auffinden geeigneter Ressourcen direkt in der Anwendung zu erleichtern.

Geeignet für folgende Anforderungen:

- Du benötigst eine geführtere Sucherfahrung
- Du möchtest schneller kompatible Dateien für ein bestimmtes Projekt sehen
- Du möchtest nicht alle Suchen außerhalb des Launchers durchführen

## lip und LeviLamina <Badge platform="windows" />

LeviLauncher enthält einen integrierten Paketmanager, der mit [lip](https://github.com/LiteLDev/lip) kompatibel ist. Du musst also weder lip noch die .NET-Runtime separat installieren. Er folgt lip: Jede Version speichert ihre Pakete in `.lip/state.json`, und Pakete werden aus ihrer `lip.toml` gelesen (ältere Pakete mit nur einer `tooth.json` funktionieren weiterhin). Versionen, in denen LeviLamina oder LIP-Pakete mit einem älteren Launcher installiert wurden (`tooth_lock.json`), werden beim ersten Paketvorgang automatisch migriert. Ist **Mirror-Quellen verwenden** aktiviert, laufen Paket-Downloads von GitHub über den **GitHub-Proxy** aus den [Einstellungen](./settings-personalization); andernfalls wird GitHub direkt erreicht (über den Systemproxy, falls einer eingerichtet ist). Ein früher gespeicherter GitHub-Proxy oder ein in lips `config.toml` (in `%APPDATA%\lip`, z. B. mit `lip config set github_proxy <URL>`) gesetzter `github_proxy` gilt in beiden Fällen. **Cache leeren** entfernt heruntergeladene Paketdateien.

Seit lip v0.40 gibt jeder Eintrag einer `lip.toml` und von `.lip/state.json` mit `side` (`server` oder `client`) und `platform` (eine .NET-Runtime-ID wie `win-x64`) an, wo er läuft; lip v0.39 und älter nannte diese Schlüssel `platform` und `runtime`. Eine `.lip/state.json` mit den alten Schlüsseln, geschrieben von lip v0.39 oder älter oder von einem älteren Launcher, wird vor dem nächsten Paketvorgang automatisch umgewandelt: `platform` wird in `side` und `runtime` in `platform` umbenannt. Eine Datei, die lip weiterhin nicht lesen kann, wird mit einer Meldung abgelehnt, die die Änderung nennt: `platform` in `side` und `runtime` in `platform` umbenennen oder die Pakete dieser Version neu installieren.

Diese Integrationen eignen sich eher für fortgeschrittene Benutzer, die paket- oder loader-gesteuerte Arbeitsabläufe verwenden möchten.

Best Practices:

- Beginne zunächst mit einer stabilen Release-Version
- Ändere nicht mehrere Hochrisikofaktoren gleichzeitig
- Dokumentiere die „zuletzt funktionierende“ Versionskombination

Um LeviLamina zu einer Client-Version hinzuzufügen, siehe [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/) in der LeviLamina-Dokumentation (auf Englisch).

### Seite „LeviMC Mods“

Die Seite **LeviMC Mods** listet die Mods von [levimc.org/mods](https://levimc.org/mods/), die einen Build für den Client unter Windows x64 haben (einen Eintrag mit `side = "client"` und `platform = "win-x64"`). Du kannst sie nach Name, ID, Autor, Tag oder Beschreibung durchsuchen, nach Relevanz, Sternen, letzter Aktualisierung oder Neuheit sortieren und nach Spielversion, LeviLamina-Version, Tags (die empfohlenen Tags wie Bibliothek / API oder Wirtschaft stehen vorne; ein Tag lässt sich auch ausschließen), Lizenz (eine Familie wie Freizügig oder eine SPDX-ID), Abhängigkeiten und Autor (**Mehr von …** auf der Seite einer Mod) filtern. Beim Installieren wird die Variante `client` gewählt, sonst die Standardvariante, sonst die erste andere Variante mit einem Client-Build; auf der Seite einer Mod kannst du eine andere dieser Varianten wählen.

## Native Mods <Badge platform="android" />

Native Mods führen nativen Code im Spielprozess aus. Installiere nur Mods aus vertrauenswürdigen Quellen und teste sie zuerst in einer isolierten Version.

Um eigene zu erstellen, folge [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) in der LeviLamina-Dokumentation (auf Englisch).

## Wenn nach dem Hinzufügen von Mods Fehler auftreten

- Deaktiviere zuerst den zuletzt hinzugefügten Mod
- Vergleiche mit dem letzten funktionierenden Zustand
- Sichere oder verschiebe wichtige Welten vor weiteren Tests
- Lies dann weiter unter [Updates und Fehlerbehebung](./update-troubleshooting)
