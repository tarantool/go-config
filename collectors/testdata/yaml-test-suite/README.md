# yaml-test-suite

The valid cases of [yaml-test-suite](https://github.com/yaml/yaml-test-suite),
the corpus YAML parsers are tested against, from its `data` branch at `6ad3d2c`
(2022-01-17). Each `<ID>.yaml` is that case's `in.yaml`, copied byte for byte;
a case with several variants, such as `4MUZ/00`, becomes `4MUZ-00.yaml`. Cases
the suite marks as errors are left out. The license is in `LICENSE`.

`tree-sitter.txt` records where tree-sitter-yaml says each node of these cases
starts and ends, for `TestYamlRanges_TreeSitter`. Regenerate it after changing
the cases:

    uv run tree_sitter_ranges.py
