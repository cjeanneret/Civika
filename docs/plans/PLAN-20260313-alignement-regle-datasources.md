# Plan d'alignement de la règle datasources

## Contexte
Le fichier `/.cursor/rules/datasources.mdc` a été enrichi récemment, mais son style et certaines formulations n'étaient pas homogènes avec les autres règles du projet (`project.mdc` et `plans.mdc`).

## Objectifs
- Harmoniser le ton et la structure avec les autres règles Cursor du dépôt.
- Rendre les consignes plus prescriptives et directement applicables.
- Supprimer les références ambiguës non résolues (ex: `[web:xx]`).

## Décisions principales
- Conserver le fond métier (sources autorisées, priorités, contraintes de sécurité).
- Aligner la forme avec une structure Markdown claire (`#`, `##`, `###`) et un frontmatter standardisé.
- Privilégier des formulations normatives ("doit", "ne doit pas", "priorité à").
- Maintenir les obligations de robustesse sécurité/données:
  - cache et limitation de requêtes,
  - tests d'intégration basés sur fixtures/mocks,
  - documentation explicite des sources et licences.

## Arborescence cible
- `/.cursor/rules/datasources.mdc`
- `docs/plans/PLAN-20260313-alignement-regle-datasources.md`

## Modifications de fichiers prévues
- `/.cursor/rules/datasources.mdc`
  - ajout d'une `description` dans le frontmatter,
  - restructuration des sections pour homogénéité,
  - reformulation normative des consignes,
  - suppression des références `[web:xx]`,
  - ajout de critères d'acceptation d'une nouvelle source.

## Contraintes de sécurité impactées
- Renforcement explicite des exigences de robustesse côté ingestion externe:
  - limitation de requêtes,
  - cache local,
  - tests sans dépendance live.
- Maintien de la traçabilité des sources (URL, organisme, licence), utile pour audit et prévention des usages non conformes.

## Vérification post-génération
- [x] Plan présent dans `docs/plans/` avec le format `PLAN-YYYYMMDD-<slug>.md`.
- [x] `datasources.mdc` conserve `alwaysApply: true`.
- [x] Le style est aligné sur les règles existantes (`project.mdc`, `plans.mdc`).
- [x] Les références ambiguës de type `[web:xx]` ont été retirées.
- [x] Les obligations de sécurité et de documentation des sources restent explicites.

