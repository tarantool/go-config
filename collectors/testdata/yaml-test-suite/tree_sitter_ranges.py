# /// script
# requires-python = ">=3.10"
# dependencies = ["tree-sitter==0.26.0", "tree-sitter-yaml==0.7.2"]
# ///
"""Writes tree-sitter.txt: where tree-sitter-yaml says each node of the
yaml-test-suite cases starts and ends, one node per line:

    <file> <kind> <start line>:<start column> <end line>:<end column>

Positions follow tree.Position: lines and columns are 1-based, columns count
runes, and the end is just past the node's last character. Nodes are listed in
document order, a collection before its children.

Run with: uv run tree_sitter_ranges.py
"""

import pathlib
import sys

import tree_sitter_yaml
from tree_sitter import Language, Parser

HERE = pathlib.Path(__file__).resolve().parent

# Nodes whose extent tree-sitter can stretch over trailing comments and line
# breaks; their real end is that of their last child that is not a comment.
LOOSE = {
    "block_node",
    "flow_node",
    "block_mapping",
    "block_sequence",
    "block_mapping_pair",
    "block_sequence_item",
    "flow_pair",
}

KINDS = {
    "block_mapping": "mapping",
    "flow_mapping": "mapping",
    "block_sequence": "sequence",
    "flow_sequence": "sequence",
    "alias": "alias",
}


def normalize(src: bytes) -> bytes:
    """Mirrors normalizeYamlSource, so offsets match yamlRanges.src."""
    if src.startswith(b"\xff\xfe"):
        src = src[2:].decode("utf-16-le").encode()
    elif src.startswith(b"\xfe\xff"):
        src = src[2:].decode("utf-16-be").encode()
    else:
        src = src.removeprefix(b"\xef\xbb\xbf")

    src = src.replace(b"\r\n", b"\n")
    for line_break in (b"\r", b"\xc2\x85", b"\xe2\x80\xa8", b"\xe2\x80\xa9"):
        src = src.replace(line_break, b"\n")

    return src


def tight_end(node, src: bytes) -> int:
    if node.type == "block_scalar":
        # At the end of the stream the token takes in the trailing blank lines.
        end = node.end_byte
        while end > node.start_byte and src[end - 1] in b" \t\n":
            end -= 1

        return end

    if node.type not in LOOSE:
        return node.end_byte

    children = [c for c in node.children if c.type != "comment"]
    if not children:
        return node.end_byte

    return tight_end(children[-1], src)


def kind(node):
    if node.type == "flow_pair":
        # A pair inside a flow sequence is a mapping of its own: `[a: b]`.
        return "mapping" if node.parent.type == "flow_sequence" else None

    if node.type not in ("block_node", "flow_node"):
        return None

    content = [c for c in node.named_children if c.type not in ("anchor", "tag", "comment")]
    if not content:
        return "scalar"

    return KINDS.get(content[0].type, "scalar")


def position(src: bytes, offset: int) -> str:
    line_start = src.rfind(b"\n", 0, offset) + 1
    line = src.count(b"\n", 0, offset) + 1
    column = len(src[line_start:offset].decode("utf-8")) + 1

    return f"{line}:{column}"


def walk(node):
    yield node
    for child in node.children:
        yield from walk(child)


def main() -> int:
    parser = Parser(Language(tree_sitter_yaml.language()))
    lines = []
    failed = []

    for path in sorted(HERE.glob("*.yaml")):
        src = normalize(path.read_bytes())
        tree = parser.parse(src)

        if tree.root_node.has_error:
            failed.append(path.name)
            continue

        for node in walk(tree.root_node):
            node_kind = kind(node)
            if node_kind is None:
                continue

            start = position(src, node.start_byte)
            end = position(src, tight_end(node, src))
            lines.append(f"{path.name} {node_kind} {start} {end}")

    (HERE / "tree-sitter.txt").write_text("\n".join(lines) + "\n")

    if failed:
        print("tree-sitter failed to parse:", " ".join(failed), file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
