# Hermes profile roster fixture provenance

These JSON files are sanitized, hand-authored contract vectors derived from
Hermes Agent `0.20.5`, tag `v2026.8.19`, commit
`fcbd1076a93841fa88855acce810e342a5b78101`, especially its
[`profiles.list` implementation](https://github.com/NousResearch/hermes-agent/blob/fcbd1076a93841fa88855acce810e342a5b78101/tui_gateway/methods_profiles.py).

They deliberately use discarded placeholder paths, models, providers,
descriptions, and UI metadata. They are not captured from a live Hermes server
and do not by themselves prove wire compatibility. A live capture remains
blocked with the real transport until Fort can bind the responding local
process to the accepted Hermes code identity described in Spec 050.

- `profiles-list-valid.json` covers nullable model/provider values and optional
  UI metadata that must be discarded.
- `profiles-list-reversed.json` covers string model/provider values, absent
  optional UI metadata, and deterministic sorting.
- `profiles-list-empty.json` covers an allocated empty successful roster.
