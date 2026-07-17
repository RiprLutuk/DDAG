#!/usr/bin/env python3
import argparse
import json


def main():
    parser = argparse.ArgumentParser(description="Generate a DDAG load-test markdown report")
    parser.add_argument("input")
    parser.add_argument("--out", default="ddag-load-report.md")
    args = parser.parse_args()

    with open(args.input, "r", encoding="utf-8") as f:
        data = json.load(f)
    s = data["summary"]
    lines = [
        "# DDAG Load Test Report",
        "",
        f"- Profile: `{data.get('profile')}`",
        f"- VUs: `{data.get('vus')}`",
        f"- Duration: `{data.get('duration_seconds')}s`",
        f"- Requests: `{s.get('requests')}`",
        f"- Success rate: `{s.get('success_rate')}%`",
        f"- Average latency: `{s.get('avg_ms')} ms`",
        f"- p95 latency: `{s.get('p95_ms')} ms`",
        "",
        "## Status Codes",
        "",
        "| Status | Count |",
        "|---:|---:|",
    ]
    for status, count in sorted(s.get("status_counts", {}).items()):
        lines.append(f"| {status} | {count} |")
    lines.append("")
    with open(args.out, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print(args.out)


if __name__ == "__main__":
    main()
