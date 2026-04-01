# Mods und Integration

LeviLauncher unterstützt Mod-bezogene Arbeitsabläufe, die über den Umfang eines „Basis-Launchers“ hinausgehen.

## Derzeit verfügbare Funktionen

- Importieren von `.zip`-Mods
- Importieren von `.dll`-Mods
- Aktivieren oder Deaktivieren installierter Mods
- Löschen nicht mehr benötigter Mods
- Nutzung von CurseForge-bezogenen Such- und Beschaffungsabläufen
- Nutzung des LIP-Paketverwaltungsablaufs
- Zusammenarbeit mit Loader-Workflows wie LeviLamina

::: warning Dies ist ein fortgeschrittener Bereich
Die Kompatibilität von Mods, Loadern und Paketmanagement-Integrationen ändert sich oft schneller, insbesondere bei Minecraft Preview. Betrachte dies als fortgeschrittene Funktion und teste schrittweise.
:::

## Empfohlener Mod-Arbeitsablauf

1. Beginne mit einer sauberen, isolierten Version.
2. Stelle zunächst sicher, dass die Version ohne Mods normal startet.
3. Füge jeweils nur einen Mod oder eine kleine Gruppe von Änderungen hinzu.
4. Teste den Start nach jeder Änderung erneut.
5. Bewahre für wichtige Welten immer einen Backup-Pfad auf.

## CurseForge

Der Launcher bietet CurseForge-bezogene Such- und Paketabläufe, um dir das Auffinden geeigneter Ressourcen direkt in der Anwendung zu erleichtern.

Geeignet für folgende Anforderungen:

- Du benötigst eine geführtere Sucherfahrung
- Du möchtest schneller kompatible Dateien für ein bestimmtes Projekt sehen
- Du möchtest nicht alle Suchen außerhalb des Launchers durchführen

## LIP und LeviLamina

Diese Integrationen eignen sich eher für fortgeschrittene Benutzer, die paket- oder loader-gesteuerte Arbeitsabläufe verwenden möchten.

Best Practices:

- Beginne zunächst mit einer stabilen Release-Version
- Ändere nicht mehrere Hochrisikofaktoren gleichzeitig
- Dokumentiere die „zuletzt funktionierende“ Versionskombination

## Wenn nach dem Hinzufügen von Mods Fehler auftreten

- Deaktiviere zuerst den zuletzt hinzugefügten Mod
- Vergleiche mit dem letzten funktionierenden Zustand
- Sichere oder verschiebe wichtige Welten vor weiteren Tests
- Lies dann weiter unter [Updates und Fehlerbehebung](./update-troubleshooting)