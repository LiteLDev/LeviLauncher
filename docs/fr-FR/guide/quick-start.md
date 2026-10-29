# Commencer en 5 minutes

Si vous voulez seulement le chemin le plus court entre le téléchargement et le jeu, suivez d'abord cette page. Choisissez **Windows** ou **Android** dans n'importe quel onglet : tout le guide suit votre choix.

## 1. Installer LeviLauncher

=== "Windows"

    - Téléchargez le programme d'installation de LeviLauncher depuis la page [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) ou le [miroir Lanzou Cloud](https://levimc.lanzoue.com/b016ke39hc). Le mot de passe pour Lanzou Cloud est `levi`.
    - Exécutez le programme d'installation sous Windows.
    - Si l'installation vous demande WebView2 ou d'autres dépendances, autorisez-la à continuer.

=== "Android"

    - Téléchargez le dernier APK depuis [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest).
    - Autorisez l'installation d'applications depuis le navigateur ou le gestionnaire de fichiers utilisé, puis installez l'APK.
    - Ouvrez LeviLauncher et acceptez le contrat du lanceur et les autorisations demandées.

## 2. Préparer Minecraft

=== "Windows"

    - Connectez-vous avec un compte Microsoft possédant une licence **Minecraft Bedrock Edition**.
    - Assurez-vous que le jeu a été installé au moins une fois depuis le Microsoft Store sur cet ordinateur.
    - Vérifiez que **Gaming Services** est disponible (nécessaire pour les versions GDK).

    ::: warning Licence officielle requise
    LeviLauncher sert à gérer Minecraft Bedrock (GDK / UWP) et ne peut pas remplacer l'autorisation du jeu.
    :::

=== "Android"

    - Installez **Minecraft Bedrock Edition** 1.21.80 ou version ultérieure depuis **Google Play** sur cet appareil.
    - LeviLauncher utilise cette copie officielle comme source de jeu de confiance ; elle ne remplace pas la licence du jeu.

## 3. Choisir une version

=== "Windows"

    - Lancez LeviLauncher.
    - Depuis la barre latérale, accédez à **Télécharger**.
    - Choisissez **Version stable** ou **Aperçu**, ainsi qu'un paquet **GDK** ou **UWP**.
    - Ici, **Version stable** et **Aperçu** désignent respectivement la build Release normale et la build Preview de Minecraft.
    - Sélectionnez la version et commencez l'installation.

=== "Android"

    - Ouvrez l'écran des versions (instances).
    - Sélectionnez la version de Minecraft à lancer, ou importez un APK officiel de Minecraft pour lancer une autre version dans le lanceur.

## 4. Décider d'activer ou non l'isolation

=== "Windows"

    L'isolation place les données de la version dans un espace de travail indépendant, au lieu de les mélanger avec l'environnement AppData par défaut. Elle est disponible pour les versions GDK et pour les versions UWP x64 1.19.70.2 ou ultérieures.

    Il est recommandé d'activer l'isolation dans les cas suivants :

    - Tester des Mods sans affecter votre environnement principal
    - Séparer clairement les versions officielles et d'aperçu de Minecraft
    - Garder des mondes et des packs de ressources distincts pour différents objectifs de jeu

=== "Android"

    Activez l'isolation des versions lorsque vous lancez une version importée dont les mondes, packs et paramètres doivent rester séparés de la version installée depuis Google Play. Les versions importées en ont souvent besoin pour démarrer.

## 5. Lancer la version installée

=== "Windows"

    - Retournez à **Lancer**.
    - Sélectionnez la version que vous venez d'installer.
    - Cliquez sur **Lancer**.

    Si LeviLauncher signale un manque de GameInput ou de Gaming Services, suivez d'abord les instructions pour installer les dépendances requises.

=== "Android"

    - Revenez à l'écran de lancement et appuyez sur **Lancer**.
    - Si le lancement échoue, ouvrez le journal de lancement depuis l'écran d'erreur et vérifiez la cause avant de modifier les paramètres.

## 6. Poursuivre la lecture des chapitres correspondants

- Préparer l'environnement : [Exigences système et installation](./requirements-installation)
- Comprendre ce qui se passe lors du premier lancement : [Premier lancement](./first-launch)
- Organiser les versions : [Gestion des versions](./version-management)
- Gérer les mondes, les packs de ressources ou les captures d'écran : [Gestion du contenu](./content-management)
- Utiliser des Mods ou des sources d'intégration : [Mods et intégrations](./mods-integrations)
