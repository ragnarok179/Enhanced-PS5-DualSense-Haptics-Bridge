# V1.50 — CI automatique et publication manuelle

Le workflow `.github/workflows/ci.yml` est automatiquement exécuté sur chaque push et pull request (et peut aussi être lancé manuellement avec `workflow_dispatch`). Il ne publie **aucune** GitHub Release, ne crée aucun tag et ne pousse aucun commit.

## Contrôles automatiques

- Environnement GitHub Actions Ubuntu avec Go **1.27.4**.
- Validation des métadonnées de compatibilité et de l'identifiant BeamNG `tagid`.
- Tests Lua, JavaScript UI, tests Go, détection de courses (`-race`) et `go vet`.
- Compilation Windows (Bridge USB, Bluetooth, launcher, updater), packaging déterministe et vérification des deux ZIPs, du bundle et des checksums.
- Compilation croisée des tests Windows USB/Bluetooth.

## Pour la release manuelle V1.50

1. Mettre à jour le dépôt GitHub avec les sources et le workflow. La CI se déclenche automatiquement au push.
2. Dans GitHub > Actions > CI, ouvrir la dernière exécution réussie **sur la branche `main`**.
3. Télécharger l'artefact `v150-release-candidate` depuis la section Artifacts de cette exécution (il s'agit d'une archive qui contient les fichiers préparés).
4. Extraire l'archive de l'artefact, puis publier manuellement une release GitHub au tag `V1.50`. Joindre **seulement** les deux ZIPs d'installation :
   - `Enhanced_PS5_DualSense_Haptics_V1.50.zip`
   - `Enhanced_PS5_DualSense_Haptics_Bridge_V1.50.zip`
5. Conserver `BRIDGE_COMPATIBILITY.json` à la racine du dépôt. Les sommes SHA256 du résultat CI figurent dans `RELEASE_SHA256SUMS.txt`, et `BUILD_PROVENANCE.txt` indique la version Go réellement utilisée.

**Important :** les EXE fournis dans les anciens paquets locaux ont été compilés avec Go 1.23.2. Pour publier des binaires compilés en Go 1.27.4, **utiliser les ZIPs de l'artefact GitHub Actions** et non les ZIPs locaux antérieurs. Une CI verte ne remplace pas les tests physiques USB/Bluetooth sur Windows.

Le dépôt GitHub actuel n'a pas été modifié dans cette préparation.

## Génération du JSON de compatibilité

La CI **ne lit pas la dernière GitHub Release publiée** pour déterminer la version à créer : elle prend celle indiquée dans `Source/internal/buildinfo/buildinfo.go` (V1.50 actuellement) et le protocole défini dans `Source/protocol.go`. Le JSON est généré automatiquement et inclus dans l'artefact CI, puis dans le ZIP Bridge. Il n'est **pas commité sur GitHub** : copier aussi le JSON de l'artefact à la racine du dépôt lors de la mise à jour manuelle.

L'ancien chemin de migration V1.42 et antérieures, l’ancien code de `UPDATE_BRIDGE.exe` et les protocoles 41/42 ont été retirés. Le nom `UPDATE_BRIDGE.exe` reste fourni comme **copie identique de l’updater moderne**, car l’updater V1.43 exige ce fichier dans l’archive téléchargée. L'updater V1.43+ reste présent. Les logs techniques du touchpad ne sont visibles qu'en mode diagnostic explicite.
