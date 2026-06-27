#!/usr/bin/env python3
import argparse
import json
import queue
import statistics
import threading
import time
import urllib.error
import urllib.request


PROFILES = {
    "low": {"vus": 5, "duration": 60},
    "medium": {"vus": 15, "duration": 120},
    "high": {"vus": 30, "duration": 120},
}


def request(base_url, token, endpoint):
    url = base_url.rstrip("/") + endpoint["path"]
    body = endpoint.get("body")
    data = None
    headers = {"Authorization": "Bearer " + token}
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, method=endpoint.get("method", "GET"), headers=headers)
    started = time.time()
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            resp.read()
            status = resp.status
    except urllib.error.HTTPError as exc:
        status = exc.code
    except Exception:
        status = 0
    return {"status": status, "latency_ms": round((time.time() - started) * 1000, 2)}


def worker(stop_at, jobs, results, base_url, token):
    while time.time() < stop_at:
        try:
            endpoint = jobs.get(timeout=0.1)
        except queue.Empty:
            continue
        results.append(request(base_url, token, endpoint))
        jobs.put(endpoint)


def summarize(results):
    latencies = [r["latency_ms"] for r in results]
    successes = [r for r in results if 200 <= r["status"] < 400]
    sorted_lat = sorted(latencies)
    p95 = sorted_lat[int(len(sorted_lat) * 0.95) - 1] if sorted_lat else 0
    return {
        "requests": len(results),
        "success": len(successes),
        "success_rate": round((len(successes) / len(results) * 100) if results else 0, 2),
        "avg_ms": round(statistics.mean(latencies), 2) if latencies else 0,
        "p95_ms": p95,
        "status_counts": {str(s): len([r for r in results if r["status"] == s]) for s in sorted({r["status"] for r in results})},
    }


def main():
    parser = argparse.ArgumentParser(description="DDAG no-dependency load tester")
    parser.add_argument("--base-url", default="http://localhost:8082")
    parser.add_argument("--token", required=True)
    parser.add_argument("--endpoints", default="tools/loadtest/endpoints.example.json")
    parser.add_argument("--profile", choices=PROFILES.keys(), default="low")
    parser.add_argument("--vus", type=int)
    parser.add_argument("--duration", type=int)
    parser.add_argument("--out", default="ddag-load-result.json")
    args = parser.parse_args()

    with open(args.endpoints, "r", encoding="utf-8") as f:
        endpoints = json.load(f)
    profile = PROFILES[args.profile]
    vus = args.vus or profile["vus"]
    duration = args.duration or profile["duration"]

    jobs = queue.Queue()
    for endpoint in endpoints:
        jobs.put(endpoint)
    results = []
    stop_at = time.time() + duration
    threads = [threading.Thread(target=worker, args=(stop_at, jobs, results, args.base_url, args.token), daemon=True) for _ in range(vus)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    payload = {"profile": args.profile, "vus": vus, "duration_seconds": duration, "summary": summarize(results), "results": results}
    with open(args.out, "w", encoding="utf-8") as f:
        json.dump(payload, f, indent=2)
    print(json.dumps(payload["summary"], indent=2))


if __name__ == "__main__":
    main()
