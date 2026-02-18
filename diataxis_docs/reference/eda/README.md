# Reference: EDA Artifacts

EDA source notebooks live in:

- `eda/autotrolej_live_eda.ipynb`
- `eda/autotrolej_static_eda.ipynb`

Raw data snapshots used by notebook workflows live in `eda/data_raw/`.

## Policy

- Notebook HTML exports are **generated artifacts** and are not canonical documentation.
- Canonical contracts belong in:
  - `../data-schema.md`
  - `../api/openapi-preview.json`

## Regeneration

Regenerate HTML exports locally from notebooks when needed for sharing or review.
Do not treat generated HTML as source-of-truth documentation.
