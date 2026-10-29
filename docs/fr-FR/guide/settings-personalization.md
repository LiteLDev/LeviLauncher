---
platforms: [windows]
---
# Paramètres & Personnalisation

LeviLauncher propose une page de paramètres assez complète pour ajuster les chemins, l'apparence, la gestion des composants, les préférences de confidentialité et la stratégie de mise à jour.

## Principales sections de paramètres

L'application divise actuellement les paramètres en plusieurs zones, par exemple :

- Paramètres généraux
- Personnalisation
- Composants tiers
- Autres
- Confidentialité
- Mises à jour
- À propos

## Ce que vous pouvez ajuster

### Chemins et stockage

- Chemin de base du contenu
- Emplacement accessible en écriture pour les fichiers gérés
- Organisation de l'environnement adaptée à la structure de vos disques

### Apparence

- Langue
- Préférence pour le thème clair ou sombre
- Comportement de la couleur principale
- Arrière-plan et autres éléments de personnalisation visuelle

#### Fond d'écran et matériau de l'interface

Choisissez un dossier d'images dans **Personnalisation → Fond d'écran → Image d'arrière-plan**, puis ouvrez **Personnalisation → Matériaux et lisibilité** pour régler le **Matériau de l'interface**, la couche située entre le fond d'écran et le contenu. La navigation, les cartes, les onglets, les contrôles de formulaire et les surfaces contextuelles partagent un même système de matériau. Les profils clair et sombre sont enregistrés séparément et appliqués automatiquement selon le thème de l'application ; les boutons **Mode clair** / **Mode sombre** de l'éditeur de matériau servent uniquement à choisir le profil à modifier et à prévisualiser.

| Paramètre | Comportement |
| --- | --- |
| Équilibré | Préréglage par défaut : verre doux avec la protection de la lisibilité activée |
| Transparent | Le fond d'écran est plus visible à travers les cartes, avec la protection de la lisibilité désactivée ; idéal avec des images simples et peu contrastées |
| Opaque | Cartes opaques sans flou de carte ; le fond d'écran reste visible en dehors des cartes |
| Opacité de l'interface | Modifie la teinte neutre des surfaces de l'interface sans estomper le texte, les icônes ou les boutons |
| Flou de l'interface | Contrôle le flou derrière les panneaux, la navigation et les surfaces contextuelles ; le flou de l'image s'applique à tout le fond d'écran |
| Voile sur le fond d'écran | Adoucit le fond d'écran avec une teinte claire en mode clair et une teinte sombre en mode sombre, indépendamment de la luminosité et de l'opacité de l'image |
| Protection de la lisibilité | Applique une teinte minimale prudente selon le thème, la luminosité, l'opacité et le flou de l'image, et renforce le texte secondaire |

L'aperçu en direct partage l'image chargée et les paramètres de rendu avec la fenêtre principale. Lorsque la protection augmente l'opacité effective des cartes, l'éditeur affiche la valeur appliquée. Ce calcul n'analyse pas l'image et ne la téléverse pas. **Réinitialiser ce thème** ne restaure que le profil de matériau en cours de modification ; il n'efface ni le dossier d'images ni le profil de l'autre thème.

Utilisez **Image suivante** lorsque le dossier contient plusieurs images. L'ordre **Séquentiel** suit le tri naturel des noms de fichiers ; l'ordre **Aléatoire** évite de répéter l'image actuelle. Lorsqu'aucune image n'est sélectionnée, que l'image ne peut pas être chargée ou que son opacité est de 0, l'interface revient aux surfaces ordinaires. Le contraste élevé, la transparence réduite et les filtres d'arrière-plan non pris en charge utilisent des surfaces de secours opaques.

### Composants tiers

- État de [lip](https://github.com/LiteLDev/lip) : le vérifier, l'installer ou le mettre à jour, et vider son cache

### Confidentialité

- Statistiques d'utilisation anonymes (Microsoft Clarity), que vous pouvez désactiver à tout moment
- Liens vers les conditions de Clarity et la déclaration de confidentialité de Microsoft

### Mises à jour

- Vérification des mises à jour
- **Recevoir les mises à jour Bêta** pour obtenir les préversions du lanceur

## Valeurs par défaut recommandées

- Placez les fichiers gérés sur un disque accessible en écriture et disposant d'un espace suffisant
- Laissez **Recevoir les mises à jour Bêta** désactivé, sauf si vous voulez les préversions du lanceur
- Stabilisez d'abord l'environnement de base avant d'affiner l'apparence

## Quand revoir les paramètres

Il est conseillé de revenir à la page des paramètres dans les cas suivants :

- Vous souhaitez déplacer l'installation vers un autre disque
- Vous rencontrez des erreurs de permission en écriture
- Vous voulez utiliser des stratégies de chemin ou d'apparence différentes sur d'autres machines
- Vous devez résoudre des problèmes liés aux téléchargements ou à la reconnaissance des composants
