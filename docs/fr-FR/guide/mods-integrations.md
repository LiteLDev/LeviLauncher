# Mods & Intégrations

LeviLauncher prend en charge des workflows liés aux Mods qui vont au-delà des fonctionnalités d'un « lanceur de base ».

## Fonctionnalités actuellement disponibles

=== "Windows"

    - Importer des paquets de Mods (`.mcmod`)
    - Les paquets `.zip` et les Mods `.dll` isolés restent acceptés : choisissez **Tous les fichiers (\*.\*)** dans le sélecteur de fichiers
    - Activer ou désactiver les Mods installés
    - Supprimer les Mods dont vous n'avez plus besoin
    - Utiliser des flux de navigation et d'obtention liés à CurseForge
    - Installer des paquets LIP avec le gestionnaire de paquets intégré, compatible avec [lip](https://github.com/LiteLDev/lip)
    - Fonctionner avec des workflows de chargeurs comme LeviLamina

    ::: warning Zone avancée
    La compatibilité des Mods, des chargeurs et des intégrations de gestion de paquets évolue souvent rapidement, surtout sur les versions Minecraft Preview (Aperçu). Considérez ceci comme une fonctionnalité avancée et procédez par tests progressifs.
    :::

=== "Android"

    - charger des Mods natifs (bibliothèques `.so`) dans le jeu
    - garder les configurations moddées dans des versions isolées

    Vous créez des Mods natifs ? Consultez [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) dans la documentation de LeviLamina (en anglais).

## Workflow recommandé pour les Mods

1. Commencez avec une version isolée propre.
2. Vérifiez d'abord que cette version peut démarrer normalement sans Mods.
3. Ajoutez un seul Mod ou un petit groupe de modifications à la fois.
4. Testez le lancement après chaque modification.
5. Conservez toujours un chemin de sauvegarde pour vos mondes importants.

## CurseForge <Badge platform="windows" />

Le lanceur propose des flux de navigation et de gestion de paquets liés à CurseForge, vous permettant de trouver plus facilement les ressources appropriées directement dans l'application.

C'est utile pour :

- Avoir une expérience de recherche plus guidée
- Voir rapidement les fichiers compatibles d'un projet
- Éviter de faire toutes vos recherches en dehors du lanceur

## lip et LeviLamina <Badge platform="windows" />

LeviLauncher intègre un gestionnaire de paquets compatible avec [lip](https://github.com/LiteLDev/lip) : vous n'avez donc pas besoin d'installer lip ni le runtime .NET séparément. Il suit lip : chaque version enregistre ses paquets dans `.lip/state.json`, et les paquets sont lus depuis leur `lip.toml` (les anciens paquets qui n'ont qu'un `tooth.json` fonctionnent toujours). Les versions sur lesquelles LeviLamina ou des paquets LIP ont été installés avec un ancien launcher (`tooth_lock.json`) sont migrées automatiquement lors de la première opération sur les paquets. Lorsque **Utiliser des miroirs** est activé, les téléchargements de paquets depuis GitHub passent par le **Proxy GitHub** défini dans les [Paramètres](./settings-personalization) ; sinon, ils se connectent directement à GitHub (via le proxy système, s'il y en a un). Un proxy GitHub enregistré auparavant, ou un `github_proxy` défini dans le `config.toml` de lip (dans `%APPDATA%\lip`, par exemple avec `lip config set github_proxy <URL>`), s'applique dans les deux cas. **Nettoyer le cache** supprime les fichiers de paquets téléchargés.

Depuis lip v0.40, chaque entrée d'un `lip.toml` et de `.lip/state.json` indique où elle s'exécute avec `side` (`server` ou `client`) et `platform` (un identifiant de runtime .NET comme `win-x64`) ; lip v0.39 et antérieur les appelait `platform` et `runtime`. Un `.lip/state.json` écrit avec les anciennes clés, par lip v0.39 ou antérieur ou par un ancien lanceur, est converti automatiquement avant la prochaine opération de paquets : `platform` devient `side` et `runtime` devient `platform`. Un fichier que lip ne peut toujours pas lire est refusé avec un message qui indique le changement : renommez `platform` en `side` et `runtime` en `platform`, ou réinstallez les paquets de cette version.

Ce type d'intégration convient mieux aux utilisateurs avancés souhaitant utiliser des workflows basés sur les paquets ou les chargeurs.

Bonnes pratiques :

- Commencez toujours par une version Release (officielle) stable
- Ne modifiez pas plusieurs facteurs à haut risque en même temps
- Notez la combinaison de versions qui « fonctionnait correctement la dernière fois »

Pour ajouter LeviLamina à une version client, consultez [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/) dans la documentation de LeviLamina (en anglais).

### Page LeviMC Mods

La page **LeviMC Mods** liste les mods de [levimc.org/mods](https://levimc.org/mods/) qui ont une version pour le client sous Windows x64 (une entrée avec `side = "client"` et `platform = "win-x64"`). Vous pouvez les rechercher par nom, ID, auteur, étiquette ou description ; les trier par pertinence, étoiles, dernière mise à jour ou plus récents ; et les filtrer par version du jeu, version de LeviLamina, étiquettes (les étiquettes recommandées comme Bibliothèque / API ou Économie viennent en premier, et une étiquette peut aussi être exclue), licence (une famille comme Permissive ou un ID SPDX), dépendances et auteur (**Plus de …** sur la page d'un mod). L'installation d'un mod choisit sa variante `client`, sinon sa variante par défaut, sinon la première autre variante ayant une version pour le client ; la page d'un mod permet de choisir une autre de ces variantes.

## Mods natifs <Badge platform="android" />

Les Mods natifs exécutent du code natif dans le processus du jeu. N'installez que des Mods de sources fiables, et testez-les d'abord dans une version isolée.

Pour créer les vôtres, suivez [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/) dans la documentation de LeviLamina (en anglais).

## Si une erreur survient après avoir ajouté des Mods

- Désactivez d'abord le Mod le plus récent
- Comparez avec l'état précédent qui fonctionnait correctement
- Transférez ou sauvegardez vos mondes importants avant de poursuivre les tests
- Consultez ensuite la page [Mise à jour et résolution de problèmes](./update-troubleshooting)
