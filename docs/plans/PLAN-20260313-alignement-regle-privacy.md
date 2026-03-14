# Plan d'alignement de la règle privacy

## Contexte
Le fichier `.cursor/rules/privacy.mdc` couvre les exigences privacy essentielles, mais sa forme doit être harmonisée avec les autres règles du projet pour garantir une application stricte et uniforme.

## Objectifs
- Aligner la langue, la structure et le registre avec `project.mdc`, `datasources.mdc` et `plans.mdc`.
- Renforcer un ton impératif sur l'ensemble des sections.
- Conserver intégralement les exigences de fond liées à la confidentialité.

## Décisions principales
- Maintenir toutes les contraintes existantes, sans assouplissement.
- Uniformiser les formulations normatives autour de verbes impératifs: "doit", "ne doit pas", "est interdit", "est obligatoire".
- Ajouter une `description` dans le frontmatter pour cohérence avec les autres règles.

## Arborescence cible
- `.cursor/rules/privacy.mdc`
- `docs/plans/PLAN-20260313-alignement-regle-privacy.md`

## Modifications de fichiers prévues
- `.cursor/rules/privacy.mdc`
  - harmonisation du frontmatter,
  - reformulation en ton impératif strict,
  - homogénéisation de la structure des sections et des listes.
- `docs/plans/PLAN-20260313-alignement-regle-privacy.md`
  - création du plan de référence et checklist de vérification.

## Contraintes de sécurité impactées
- Maintien strict du modèle "calculer et jeter".
- Interdiction explicite de la persistance des données personnelles.
- Interdiction explicite d'envoyer des données identifiantes aux services LLM.
- Limitation des logs à des métadonnées techniques non personnelles.

## Vérification post-génération
- [x] Le plan existe dans `docs/plans/` au format `PLAN-YYYYMMDD-<slug>.md`.
- [x] Le ton de `privacy.mdc` est impératif et uniforme sur toutes les sections.
- [x] Le contenu reste cohérent avec les autres règles Cursor du dépôt.
- [x] Aucune régression des exigences privacy/sécurité.
