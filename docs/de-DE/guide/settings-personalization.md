---
platforms: [windows]
---
# Einstellungen & Personalisierung

LeviLauncher bietet eine umfassende Einstellungsseite, auf der du Pfade, Erscheinungsbild, Komponentenverwaltung, Datenschutzeinstellungen und das Update-Verhalten anpassen kannst.

## Hauptbereiche der Einstellungen

Die Anwendung unterteilt die Einstellungen derzeit in mehrere Bereiche, zum Beispiel:

- Allgemeine Einstellungen
- Personalisierung
- Drittanbieter-Komponenten
- Sonstiges
- Datenschutz
- Updates
- Über

## Was du anpassen kannst

### Pfade und Speicherung

- Basispfad für Inhalte
- Beschreibbarer Speicherort für verwaltete Dateien
- Eine für deine Festplattenstruktur besser geeignete Platzierung

### Erscheinungsbild

- Sprache
- Präferenz für helles oder dunkles Thema
- Verhalten der Akzentfarbe
- Hintergrund- und visuelle Personalisierungsoptionen

#### Hintergrundbild und Oberflächenmaterial

Wähle unter **Personalisierung → Hintergrundbild** einen Bildordner aus und öffne dann **Personalisierung → Material und Lesbarkeit**, um das **Oberflächenmaterial** anzupassen – die Ebene zwischen Hintergrundbild und Inhalt. Navigation, Karten, Tabs, Formularelemente und Popup-Flächen nutzen ein gemeinsames Materialsystem. Helle und dunkle Profile werden getrennt gespeichert und automatisch passend zum App-Design angewendet; die Schaltflächen **Hellmodus** / **Dunkelmodus** im Material-Editor wählen nur das Profil aus, das bearbeitet und in der Vorschau angezeigt wird.

| Einstellung | Verhalten |
| --- | --- |
| Ausgewogen | Standardvorgabe: weiches Glas mit eingeschaltetem Lesbarkeitsschutz |
| Klar | Mehr Hintergrundbild scheint durch die Karten, Lesbarkeitsschutz ist ausgeschaltet; am besten mit einfachen, kontrastarmen Bildern |
| Deckend | Undurchsichtige Karten ohne Kartenunschärfe; außerhalb der Karten bleibt das Hintergrundbild sichtbar |
| Deckkraft der Oberfläche | Ändert die neutrale Tönung der Oberflächen, ohne Text, Symbole oder Schaltflächen verblassen zu lassen |
| Unschärfe der Oberfläche | Steuert die Unschärfe hinter Panels, Navigation und Popup-Flächen; die Bildunschärfe wirkt auf das gesamte Hintergrundbild |
| Hintergrundbild-Overlay | Dämpft das Hintergrundbild im Hellmodus mit einer hellen und im Dunkelmodus mit einer dunklen Tönung, unabhängig von Bildhelligkeit und Deckkraft |
| Lesbarkeitsschutz | Wendet abhängig von Design, Helligkeit, Bilddeckkraft und Unschärfe eine zurückhaltende Mindesttönung an und verstärkt sekundären Text |

Die Live-Vorschau verwendet dasselbe geladene Bild und dieselben Rendering-Parameter wie das Hauptfenster. Wenn der Schutz die effektive Deckkraft der Karten erhöht, zeigt der Editor den angewendeten Wert an. Diese Berechnung analysiert das Bild nicht und lädt es nicht hoch. **Dieses Design zurücksetzen** setzt nur das gerade bearbeitete Materialprofil zurück; der Bildordner und das Profil des anderen Designs bleiben unverändert.

Verwende **Nächstes Bild**, wenn der Ordner mehrere Bilder enthält. Die Reihenfolge **Sequentiell** nutzt eine natürliche Sortierung nach Dateinamen; **Zufällig** vermeidet, das aktuelle Bild zu wiederholen. Wenn kein Bild ausgewählt ist, das Bild nicht geladen werden kann oder die Bilddeckkraft 0 beträgt, kehrt die Oberfläche zu normalen Flächen zurück. Bei erhöhtem Kontrast, reduzierter Transparenz oder nicht unterstützten Backdrop-Filtern werden deckende Ersatzflächen verwendet.

### Drittanbieter-Komponenten

- Status von [lip](https://github.com/LiteLDev/lip): prüfen, installieren oder aktualisieren sowie den Cache leeren

### Datenschutz

- Anonyme Nutzungsanalyse (Microsoft Clarity), die du jederzeit ausschalten kannst
- Links zu den Clarity-Nutzungsbedingungen und der Microsoft-Datenschutzerklärung

### Updates

- Überprüfung auf Updates
- **Beta-Updates erhalten**, um Vorabversionen des Launchers zu bekommen

## Empfohlene Standardeinstellungen

- Speichere verwaltete Dateien an einem beschreibbaren und speicherplatzreichen Ort auf der Festplatte
- Lass **Beta-Updates erhalten** ausgeschaltet, sofern du keine Vorabversionen des Launchers möchtest
- Stelle zunächst die Grundumgebung stabil ein, bevor du das Erscheinungsbild feinjustierst

## Wann es sich lohnt, die Einstellungen zu überprüfen

In folgenden Situationen empfiehlt es sich, die Einstellungsseite erneut zu prüfen:

- Wenn du die Installation auf eine andere Festplatte verschieben möchtest
- Bei Schreibberechtigungsfehlern
- Wenn du auf verschiedenen Computern unterschiedliche Erscheinungsbilder oder Pfadstrategien verwenden möchtest
- Zur Fehlerbehebung bei Download- oder Komponentenerkennungsproblemen
