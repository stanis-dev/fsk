#!/usr/bin/env python3
"""Resolve $refs in the fiskaly unified OAS and print required-field skeletons
for the create-request schemas, so probe payloads can be built precisely."""
import sys
import yaml

SPEC = "research/specs/fiskaly_oas_2026-02-03.yaml"
doc = yaml.safe_load(open(SPEC))
SCHEMAS = doc["components"]["schemas"]


def deref(node):
    while isinstance(node, dict) and "$ref" in node:
        node = SCHEMAS[node["$ref"].rsplit("/", 1)[1]]
    return node


def skeleton(node, depth=0, optional=False, seen=None):
    """Build a skeleton showing required fields (and one level of optionals)."""
    seen = seen or set()
    node = deref(node)
    if id(node) in seen:
        return "<recursion>"
    seen = seen | {id(node)}

    if "oneOf" in node:
        disc = node.get("discriminator", {})
        variants = {}
        for ref in node["oneOf"]:
            name = ref.get("$ref", "?").rsplit("/", 1)[-1]
            variants[f"oneOf:{name}"] = skeleton(ref, depth + 1, optional, seen)
        return variants
    if "allOf" in node:
        merged = {}
        for part in node["allOf"]:
            s = skeleton(part, depth, optional, seen)
            if isinstance(s, dict):
                merged.update(s)
            else:
                return s
        return merged

    t = node.get("type")
    if t == "object":
        req = node.get("required", [])
        props = node.get("properties", {})
        out = {}
        for k, v in props.items():
            if k in req:
                out[k] = skeleton(v, depth + 1, optional, seen)
            elif depth < 3:
                out[f"{k}?"] = skeleton(v, depth + 1, True, seen)
        return out
    if t == "array":
        return [skeleton(node.get("items", {}), depth + 1, optional, seen)]
    if "enum" in node:
        return "|".join(str(e) for e in node["enum"])
    desc = []
    if t:
        desc.append(t)
    for key in ("pattern", "format", "minLength", "maxLength", "minimum", "maximum"):
        if key in node:
            desc.append(f"{key}={node[key]}")
    if "example" in node:
        desc.append(f"e.g. {node['example']}")
    return " ".join(desc) or "?"


for name in sys.argv[1:]:
    print(f"\n{'='*70}\n### {name}")
    print(yaml.dump(skeleton(SCHEMAS[name]), default_flow_style=False, sort_keys=False, allow_unicode=True))
