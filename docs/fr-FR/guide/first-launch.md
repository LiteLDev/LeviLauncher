# Premier lancement

À la première ouverture, LeviLauncher vérifie que votre appareil et Minecraft sont prêts pour la gestion des versions.

## Ce que vous pourriez voir

=== "Windows"

    Lors du premier lancement, LeviLauncher pourrait :

    - Vérifier la disponibilité de Gaming Services (GDK)
    - Vérifier la disponibilité de GameInput (GDK)
    - Vérifier que le runtime Visual C++ requis par le chargeur de Mods natif est installé
    - Valider que l'environnement Minecraft Bedrock (GDK / UWP) est prêt
    - Afficher un guide ou une alerte en cas de composants essentiels manquants

=== "Android"

    Au premier lancement, LeviLauncher peut :

    - vous demander d'accepter le contrat du lanceur
    - demander les autorisations nécessaires
    - vérifier que le Minecraft officiel de Google Play (1.21.80 ou version ultérieure) est installé
    - migrer les répertoires de données d'une ancienne version du lanceur

## Procédure courante du premier lancement

=== "Windows"

    1. Ouvrez LeviLauncher.
    2. Lisez les informations de guide fournies par le lanceur.
    3. Installez ou réparez les composants préalables manquants.
    4. Retournez dans le lanceur pour revérifier l'état.
    5. Une fois les conditions remplies, accédez à la page **Télécharger**.

=== "Android"

    1. Ouvrez LeviLauncher.
    2. Acceptez le contrat et accordez les autorisations demandées.
    3. Si le lanceur migre d'anciennes données, attendez la fin. Ne supprimez pas l'ancien répertoire tant que vos mondes, packs et paramètres ne sont pas de nouveau disponibles.
    4. Passez à l'écran des versions.

## Si des composants manquent <Badge platform="windows" />

### Gaming Services

Si Gaming Services est manquant ou endommagé, LeviLauncher pourrait vous guider vers le Microsoft Store pour l'installer ou le réparer.

### GameInput

Si GameInput est manquant, installez le redistribuable requis selon les instructions.

### Minecraft Bedrock non détecté

Veuillez confirmer que :

- Le compte Microsoft actuel possède une licence du jeu
- Le jeu a déjà été installé au moins une fois depuis le Microsoft Store sur cet ordinateur
- Vous n'essayez pas de contourner le processus d'autorisation officiel en utilisant le lanceur

## Bonnes valeurs par défaut pour les nouveaux utilisateurs

- Commencez par utiliser une version Release (officielle) isolée
- Placez les chemins de téléchargement et de contenu sur un disque accessible en écriture
- N'importez pas de nombreux Mods avant d'avoir réussi un premier lancement propre

## Après le premier lancement, nous vous recommandons de lire

- [Gestion des versions](./version-management)
- [Gestion du contenu](./content-management)
- [Paramètres et personnalisation](./settings-personalization)
