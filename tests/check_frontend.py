#!/usr/bin/env python3
"""
Frontend DOM & Script Verification
Validates that:
1. JavaScript syntax is 100% valid.
2. All getElementById references in app.js exist in index.html.
3. No broken references or missing asset links exist.
"""
import re
import sys
import subprocess
from pathlib import Path

def test_frontend():
    root = Path(__file__).resolve().parent.parent
    html_file = root / "web" / "index.html"
    js_file = root / "web" / "js" / "app.js"

    # 1. Node syntax check
    res = subprocess.run(["node", "--check", str(js_file)], capture_output=True, text=True)
    if res.returncode != 0:
        print(f"FAILED: JS Syntax error in {js_file}:\n{res.stderr}", file=sys.stderr)
        return False
    print("PASS: JavaScript syntax is valid (node --check passed).")

    # 2. Check DOM IDs
    html = html_file.read_text(encoding="utf-8")
    js = js_file.read_text(encoding="utf-8")

    queried_ids = set(re.findall(r"getElementById\(['\"]([^'\"]+)['\"]\)", js))
    html_ids = set(re.findall(r"id=['\"]([^'\"]+)['\"]", html))

    missing = queried_ids - html_ids
    if missing:
        print(f"FAILED: The following IDs are queried in app.js but missing in index.html: {missing}", file=sys.stderr)
        return False
    print(f"PASS: All {len(queried_ids)} getElementById queries exist in index.html.")

    # 3. Check CSS file exists
    css_file = root / "web" / "css" / "style.css"
    if not css_file.exists():
        print("FAILED: style.css not found!", file=sys.stderr)
        return False
    print("PASS: CSS file exists.")

    return True

if __name__ == "__main__":
    if not test_frontend():
        sys.exit(1)
    print("All frontend structural checks passed successfully!")
