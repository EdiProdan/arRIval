# arRIval

## EDA notebook (AUTOTROLEJ static data)

Notebook: `eda/autotrolej_static_eda.ipynb`

What it does:
- downloads static AUTOTROLEJ JSON sources (`ATlinije`, `ATstanice`, `ATvoznired*`)
- caches raw files into `eda/data_raw/`
- runs MVP exploratory checks (schema, nulls, duplicates, consistency)

How to run:
1. Open `eda/autotrolej_static_eda.ipynb` in VS Code.
2. Select a Python kernel.
3. Run all cells from top to bottom.

Cache behavior:
- by default, notebook reads from cache when files already exist
- set `REFRESH_CACHE = True` in the configuration cell to re-download all sources