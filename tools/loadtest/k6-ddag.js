import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

export const latency = new Trend('ddag_latency_ms');
export const successRate = new Rate('ddag_success_rate');

const profile = __ENV.PROFILE || 'low';
const profiles = {
  low: { vus: 5, duration: '1m' },
  medium: { vus: 15, duration: '2m' },
  high: { vus: 30, duration: '2m' },
};
const selectedProfile = profiles[profile] || profiles.low;

export const options = {
  scenarios: {
    ddag: {
      executor: 'constant-vus',
      vus: selectedProfile.vus,
      duration: selectedProfile.duration,
    },
  },
  thresholds: {
    ddag_success_rate: ['rate>=0.80'],
    http_req_failed: ['rate<0.20'],
  },
};

const baseURL = (__ENV.BASE_URL || 'http://localhost:8082').replace(/\/$/, '');
const token = __ENV.TOKEN || '';
const endpoints = JSON.parse(__ENV.ENDPOINTS || '[{"method":"GET","path":"/openapi.json"}]');

export default function () {
  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const params = { headers: { Authorization: `Bearer ${token}` } };
  if (endpoint.body) params.headers['Content-Type'] = 'application/json';
  const started = Date.now();
  const res = endpoint.method === 'POST'
    ? http.post(baseURL + endpoint.path, JSON.stringify(endpoint.body || {}), params)
    : http.get(baseURL + endpoint.path, params);
  latency.add(Date.now() - started);
  const ok = check(res, {
    'status < 500': (r) => r.status < 500,
    'structured json': (r) => (r.headers['Content-Type'] || '').includes('application/json'),
  });
  successRate.add(ok && res.status >= 200 && res.status < 400);
  sleep(1);
}
