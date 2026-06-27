#!/usr/bin/env python3
import argparse
import json
import urllib.request


def main():
    parser = argparse.ArgumentParser(description="Export DDAG /api-catalog to load-test endpoint JSON")
    parser.add_argument("--gateway-url", default="http://localhost:8082")
    parser.add_argument("--token", default="")
    parser.add_argument("--out", default="ddag-endpoints.json")
    args = parser.parse_args()

    req = urllib.request.Request(args.gateway_url.rstrip("/") + "/api-catalog")
    if args.token:
        req.add_header("Authorization", "Bearer " + args.token)
    with urllib.request.urlopen(req, timeout=30) as resp:
        env = json.loads(resp.read().decode())
    apis = env.get("data") or []
    endpoints = [{"method": api.get("method", "GET"), "path": api.get("path", "")} for api in apis if api.get("path")]
    with open(args.out, "w", encoding="utf-8") as f:
        json.dump(endpoints, f, indent=2)
    print(args.out)


if __name__ == "__main__":
    main()
