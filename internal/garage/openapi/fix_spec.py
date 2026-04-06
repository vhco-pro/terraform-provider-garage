#!/usr/bin/env python3
"""Convert the Garage OpenAPI 3.1 spec to 3.0 compatible form for oapi-codegen.

Fixes:
- "items": false → "items": {} (empty schema)
- "type": ["string", "null"] → "type": "string", "nullable": true
- "openapi": "3.1.0" → "openapi": "3.0.3"
- oneOf with {"type": "null"} → nullable on the other branch
"""
import json
import sys


def fix_spec(obj):
    """Recursively fix OpenAPI 3.1 constructs to 3.0 equivalents."""
    if isinstance(obj, dict):
        # Fix boolean items
        if "items" in obj and isinstance(obj["items"], bool):
            obj["items"] = {}

        # Fix type arrays: ["string", "null"] → "string" + nullable
        if "type" in obj and isinstance(obj["type"], list):
            types = obj["type"]
            non_null = [t for t in types if t != "null"]
            if "null" in types and len(non_null) == 1:
                obj["type"] = non_null[0]
                obj["nullable"] = True
            elif len(non_null) > 1:
                # Multiple non-null types — keep first as best effort
                obj["type"] = non_null[0]
            elif not non_null:
                obj["type"] = "string"
                obj["nullable"] = True

        # Fix oneOf with null type: [{type: null}, {$ref or schema}] → nullable schema
        if "oneOf" in obj and isinstance(obj["oneOf"], list):
            one_of = obj["oneOf"]
            null_branches = [b for b in one_of if isinstance(b, dict) and b.get("type") == "null"]
            non_null_branches = [b for b in one_of if b not in null_branches]
            if len(null_branches) == 1 and len(non_null_branches) == 1:
                # Replace oneOf with the non-null branch + nullable
                branch = non_null_branches[0]
                del obj["oneOf"]
                obj.update(branch)
                obj["nullable"] = True

        # Recurse into all values
        for k, v in list(obj.items()):
            fix_spec(v)

    elif isinstance(obj, list):
        for item in obj:
            fix_spec(item)


def main():
    input_path = sys.argv[1] if len(sys.argv) > 1 else "openapi/spec.json"
    output_path = sys.argv[2] if len(sys.argv) > 2 else "openapi/spec.processed.json"

    with open(input_path) as f:
        spec = json.load(f)

    # Downgrade version
    spec["openapi"] = "3.0.3"

    fix_spec(spec)

    with open(output_path, "w") as f:
        json.dump(spec, f, indent=2)
        f.write("\n")

    print(f"Processed spec written to {output_path}", file=sys.stderr)


if __name__ == "__main__":
    main()
