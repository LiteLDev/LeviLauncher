# Mods & Integrationen

LeviLauncher unterstützt Mod-bezogene Arbeitsabläufe, die über den Umfang eines „Basis-Launchers“ hinausgehen.

## Derzeit verfügbare Funktionen

=== "Windows"

    - Importieren von `.zip`-Mods
    - Importieren von `.dll`-Mods
    - Aktivieren oder Deaktivieren installierter Mods
    - Löschen nicht mehr benötigter Mods
    - Nutzung von CurseForge-bezogenen Such- und Beschaffungsabläufen
    - Installieren von Paketen mit [lip](https://github.com/LiteLDev/lip), dem Paketinstaller
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

Diese Integrationen eignen sich eher für fortgeschrittene Benutzer, die paket- oder loader-gesteuerte Arbeitsabläufe verwenden möchten.

Best Practices:

- Beginne zunächst mit einer stabilen Release-Version
- Ändere nicht mehrere Hochrisikofaktoren gleichzeitig
- Dokumentiere die „zuletzt funktionierende“ Versionskombination

Um LeviLamina zu einer Client-Version hinzuzufügen, siehe [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/) in der LeviLamina-Dokumentation (auf Englisch).

## Native Mods <Badge platform="android" />

Native Mods führen nativen Code im Spielprozess aus. Installiere nur Mods aus vertrauenswürdigen Quellen und teste sie zuerst in einer isolierten Version.

Um eigene zu erstellen, folge [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) in der LeviLamina-Dokumentation (auf Englisch).

## Wenn nach dem Hinzufügen von Mods Fehler auftreten

- Deaktiviere zuerst den zuletzt hinzugefügten Mod
- Vergleiche mit dem letzten funktionierenden Zustand
- Sichere oder verschiebe wichtige Welten vor weiteren Tests
- Lies dann weiter unter [Updates und Fehlerbehebung](./update-troubleshooting)
