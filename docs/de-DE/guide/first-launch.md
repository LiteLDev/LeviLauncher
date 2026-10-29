# Erster Start

Beim ersten Öffnen prüft LeviLauncher, ob dein Gerät und Minecraft für verwaltete Versionen bereit sind.

## Was du möglicherweise siehst

=== "Windows"

    Beim ersten Start kann LeviLauncher Folgendes tun:

    - Überprüfen, ob Gaming Services verfügbar ist (GDK)  
    - Überprüfen, ob GameInput verfügbar ist (GDK)  
    - Überprüfen, ob die vom nativen Mod-Loader benötigte Visual C++-Laufzeit installiert ist  
    - Validieren, ob die Minecraft Bedrock (GDK / UWP)-Umgebung bereit ist  
    - Bei fehlenden Schlüsselkomponenten entsprechende Hinweise oder Anleitungen anzeigen  

=== "Android"

    Beim ersten Start kann LeviLauncher:

    - dich bitten, die Launcher-Vereinbarung zu akzeptieren
    - die benötigten Berechtigungen anfragen
    - prüfen, ob das offizielle Minecraft aus Google Play (1.21.80 oder neuer) installiert ist
    - Datenverzeichnisse einer älteren Launcher-Version migrieren

## Typischer Ablauf beim ersten Start

=== "Windows"

    1. Öffne LeviLauncher.  
    2. Lies die angezeigten Hinweise des Launchers.  
    3. Installiere oder repariere fehlende Voraussetzungen.  
    4. Kehre zum Launcher zurück und überprüfe den Status erneut.  
    5. Sobald alle Bedingungen erfüllt sind, gelangst du zur Seite **Herunterladen**.

=== "Android"

    1. Öffne LeviLauncher.
    2. Akzeptiere die Vereinbarung und erteile die angefragten Berechtigungen.
    3. Migriert der Launcher alte Daten, warte, bis er fertig ist. Lösche das alte Verzeichnis erst, wenn deine Welten, Pakete und Einstellungen wieder verfügbar sind.
    4. Weiter zur Versionsansicht.

## Falls Komponenten als fehlend gemeldet werden <Badge platform="windows" />

### Gaming Services

Falls Gaming Services fehlt oder beschädigt ist, leitet dich LeviLauncher möglicherweise zum Microsoft Store, um es zu installieren oder zu reparieren.

### GameInput

Falls GameInput fehlt, installiere bitte das angeforderte Redistributable-Paket gemäß den Anweisungen.

### Minecraft Bedrock wurde nicht erkannt

Bitte stelle sicher, dass:

- Dein aktuelles Microsoft-Konto über eine gültige Spiellizenz verfügt  
- Das Spiel auf diesem Computer bereits über den Microsoft Store installiert wurde  
- Du nicht versuchst, den offiziellen Lizenzierungsprozess zu umgehen  

## Empfohlene Standardeinstellungen für neue Nutzer

- Beginne zunächst mit einer isolierten **Release**-Version  
- Lege Download- und Inhaltspfade auf ein beschreibbares Laufwerk  
- Importiere keine große Anzahl von Mods, bevor der erste saubere Start erfolgreich war  

## Empfohlene weiterführende Themen nach dem ersten Start

- [Versionsverwaltung](./version-management)  
- [Inhaltsverwaltung](./content-management)  
- [Einstellungen und Personalisierung](./settings-personalization)
