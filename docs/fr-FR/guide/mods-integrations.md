# Mods et intégrations

LeviLauncher prend en charge des workflows liés aux Mods qui vont au-delà des fonctionnalités d'un « lanceur de base ».

## Fonctionnalités actuellement disponibles

- Importer des Mods `.zip`
- Importer des Mods `.dll`
- Activer ou désactiver les Mods installés
- Supprimer les Mods dont vous n'avez plus besoin
- Utiliser des flux de navigation et d'obtention liés à CurseForge
- Utiliser le gestionnaire de paquets LIP
- Fonctionner avec des workflows de chargeurs comme LeviLamina

::: warning Zone avancée
La compatibilité des Mods, des chargeurs et des intégrations de gestion de paquets évolue souvent rapidement, surtout sur les versions Minecraft Preview (Aperçu). Considérez ceci comme une fonctionnalité avancée et procédez par tests progressifs.
:::

## Workflow recommandé pour les Mods

1. Commencez avec une version isolée propre.
2. Vérifiez d'abord que cette version peut démarrer normalement sans Mods.
3. Ajoutez un seul Mod ou un petit groupe de modifications à la fois.
4. Testez le lancement après chaque modification.
5. Conservez toujours un chemin de sauvegarde pour vos mondes importants.

## CurseForge

Le lanceur propose des flux de navigation et de gestion de paquets liés à CurseForge, vous permettant de trouver plus facilement les ressources appropriées directement dans l'application.

C'est utile pour :

- Avoir une expérience de recherche plus guidée
- Voir rapidement les fichiers compatibles d'un projet
- Éviter de faire toutes vos recherches en dehors du lanceur

## LIP et LeviLamina

Ce type d'intégration convient mieux aux utilisateurs avancés souhaitant utiliser des workflows basés sur les paquets ou les chargeurs.

Bonnes pratiques :

- Commencez toujours par une version Release (officielle) stable
- Ne modifiez pas plusieurs facteurs à haut risque en même temps
- Notez la combinaison de versions qui « fonctionnait correctement la dernière fois »

## Si une erreur survient après avoir ajouté des Mods

- Désactivez d'abord le Mod le plus récent
- Comparez avec l'état précédent qui fonctionnait correctement
- Transférez ou sauvegardez vos mondes importants avant de poursuivre les tests
- Consultez ensuite la page [Mise à jour et résolution de problèmes](./update-troubleshooting)