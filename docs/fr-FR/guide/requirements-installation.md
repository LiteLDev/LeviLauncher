# Configuration requise & Installation

Cette page explique ce dont LeviLauncher a besoin avant de pouvoir installer et lancer Minecraft Bedrock Edition, sous Windows et sous Android.

Les versions UWP nécessitent le mode développeur de Windows ainsi que les dépendances du framework UWP déclarées par le paquet (comme Microsoft.VCLibs). Les exigences Gaming Services et GameInput ci-dessous s'appliquent à GDK. L'isolation des instances fonctionne pour GDK et pour les paquets UWP x64 de version 1.19.70.2 ou ultérieure ; les instances UWP non isolées partagent le contenu `LocalState` de l'application Windows. Les instances UWP x64 prennent aussi en charge la console et les Mods DLL `preload-native` compatibles, mais l'installation automatique de LeviLamina n'est disponible que pour les versions GDK prises en charge.

## Exigences système

=== "Windows"

    | Élément | Exigence |
    | --- | --- |
    | Système d'exploitation | Windows 10 ou Windows 11 |
    | Version du jeu | Minecraft Bedrock Edition (GDK / UWP) |
    | Licence | Licence officielle liée à un compte Microsoft |
    | Réseau | Nécessaire pour télécharger les versions, obtenir les métadonnées, tester la vitesse des miroirs et vérifier les mises à jour |

=== "Android"

    | Élément | Exigence |
    | --- | --- |
    | Système d'exploitation | Android 9.0 (API 28) ou version ultérieure |
    | Architecture | ARM64 (arm64-v8a) |
    | Mémoire | 1 Go de RAM disponible (3 Go ou plus recommandés) |
    | Stockage | 2 Go libres pour Minecraft et les données du jeu (5 Go ou plus recommandés) |
    | Jeu | Minecraft Bedrock Edition 1.21.80 ou version ultérieure, installé depuis Google Play |
    | Licence | Une copie légitime de Minecraft achetée sur Google Play |

## Composants Windows nécessaires <Badge platform="windows" />

Avant le premier lancement ou l'installation, LeviLauncher pourrait vous guider pour installer les composants manquants.

- **Microsoft Gaming Services** (GDK)
- **Microsoft GameInput** (GDK)
- **WebView2 Runtime**
- **Microsoft Visual C++ Redistributable (x64)**, requis par le chargeur de Mods natif ; LeviLauncher vérifie sa présence et propose de l'installer

La présence ou non de ces composants dépend de l'état de votre environnement Windows.

### Utiliser un WebView2 Fixed Version Runtime personnalisé

Si le WebView2 Runtime Evergreen n'est pas installé mais que vous gérez vous-même un WebView2 Fixed Version Runtime, démarrez LeviLauncher avec l'une des méthodes suivantes.

Indiquez le répertoire en ligne de commande :

```powershell
LeviLauncher.exe --webview2-runtime-dir "D:\WebView2\FixedRuntime"
```

Ou créez un fichier `webview2.json` à côté de `LeviLauncher.exe` :

```json
{
  "browserExecutableFolder": "D:\\WebView2\\FixedRuntime"
}
```

Les chemins relatifs sont résolus à partir du répertoire contenant `webview2.json`. Le répertoire choisi doit contenir directement `msedgewebview2.exe`. L'option de ligne de commande est prioritaire sur le fichier de configuration. Si aucune des deux n'est configurée, LeviLauncher continue d'utiliser le WebView2 Runtime installé sur le système.

## Avant d'installer une version

=== "Windows"

    Veuillez compléter cette liste de vérification :

    1. Avoir installé Minecraft Bedrock au moins une fois depuis le Microsoft Store.
    2. Si l'état du Store est anormal, lancez le jeu une fois pour confirmer que l'installation est complète.
    3. Fermez le jeu avant d'utiliser LeviLauncher pour installer ou gérer des versions.

=== "Android"

    1. Installez Minecraft Bedrock Edition depuis Google Play sur l'appareil.
    2. Mettez-le à jour vers 1.21.80 ou version ultérieure : le lanceur refuse les versions plus anciennes.
    3. Vérifiez qu'il reste assez d'espace pour les versions et les mondes que vous voulez conserver.

## Installer LeviLauncher lui-même

=== "Windows"

    ### Option A : Page GitHub Releases

    Convient aux utilisateurs souhaitant télécharger directement depuis la page officielle de LeviLauncher et consulter l'historique des mises à jour.

    1. Ouvrez la page [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) de LeviLauncher.
    2. Téléchargez le programme d'installation.
    3. Exécutez-le et suivez l'assistant d'installation.

    ### Option B : Miroir Lanzou Cloud

    Si vous avez une connexion lente à GitHub depuis votre région, ce lien est généralement plus pratique.

    1. Ouvrez [Lanzou Cloud](https://levimc.lanzoue.com/b016ke39hc).
    2. Saisissez le mot de passe `levi`.
    3. Téléchargez le fichier et exécutez le programme d'installation localement.

=== "Android"

    1. Ouvrez la page [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) de la version Android.
    2. Téléchargez le dernier APK.
    3. Autorisez l'installation depuis le navigateur ou le gestionnaire de fichiers utilisé pour le télécharger.
    4. Installez l'APK et ouvrez-le (l'application s'appelle « LeviLauncher »).
    5. Acceptez le contrat du lanceur et accordez les autorisations demandées par l'application.

## Installer une première version gérée

=== "Windows"

    1. Dans LeviLauncher, ouvrez **Télécharger**.
    2. Choisissez la build Minecraft **Version stable** (Release) ou **Aperçu** (Preview) que vous souhaitez installer, ainsi que le type de paquet **GDK** ou **UWP**.
    3. Sélectionnez l'entrée de la version cible.
    4. Décidez d'activer ou non l'isolation.
    5. Lancez l'installation et attendez sa fin.

=== "Android"

    1. Ouvrez l'écran des versions (instances).
    2. Sélectionnez la version installée depuis Google Play, ou importez un APK officiel de Minecraft.
    3. Activez l'isolation pour une version importée dont les données doivent rester séparées.
    4. Revenez à l'écran de lancement et appuyez sur **Lancer**.

## Stratégie d'installation recommandée <Badge platform="windows" />

### Quand choisir la version Release (officielle)

- Vous souhaitez un environnement de jeu quotidien plus stable
- Vous gérez des mondes à long terme
- Vous préférez que les Mods et les packs de ressources changent moins souvent

### Quand choisir la version Preview (Aperçu)

- Vous voulez tester les futures fonctionnalités en avance
- Vous acceptez les instabilités ou les changements de compatibilité
- Vous êtes prêt à séparer clairement votre environnement Preview de votre environnement de jeu quotidien

::: tip Pratique recommandée pour la majorité des joueurs
Commencez par créer une **version Release (officielle) isolée**. N'ajoutez une **version Preview (Aperçu)** que lorsque vous avez clairement besoin de tester le contenu à venir.
:::

## Si l'installation ne peut pas se poursuivre

=== "Windows"

    Les problèmes suivants peuvent être résolus en consultant la page [Mise à jour et résolution de problèmes](./update-troubleshooting) :

    - Manque de Gaming Services
    - Manque de GameInput
    - État incomplet de la licence ou de l'installation via le Store
    - Chemin de destination non accessible en écriture
    - Échec du téléchargement ou du miroir

=== "Android"

    - Si le lanceur indique que Minecraft est absent ou non pris en charge, installez ou mettez à jour d'abord la version Google Play.
    - Les versions antérieures à 1.21.80 ne sont pas prises en charge.
    - Pour les échecs de lancement, consultez [Mises à jour & Dépannage](./update-troubleshooting).
