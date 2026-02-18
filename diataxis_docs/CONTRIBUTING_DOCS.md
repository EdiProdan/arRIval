# Documentation Contribution Guide

Use this checklist when adding or updating documentation.

## Choose the right Diátaxis section

- Put it in **Tutorials** if it teaches through a guided learning path.
- Put it in **How-to** if it solves a specific operational/development task.
- Put it in **Reference** if it defines factual contracts, schemas, APIs, or exact behavior.
- Put it in **Explanation** if it describes rationale, trade-offs, and architecture reasoning.

## Authoring rules

- Prefer concise headings and command-first instructions for developer-facing docs.
- Keep examples copy-pastable.
- Link to canonical files in `diataxis_docs/` rather than legacy files in `docs/`.
- When moving content, preserve technical accuracy before improving style.

## Update checklist

1. Update or add the target Diátaxis file.
2. Update `diataxis_docs/README.md` navigation if a new page is added.
3. Verify links from root `README.md` remain valid.
4. If a factual contract changed, update corresponding Reference pages.
