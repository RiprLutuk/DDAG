<script setup lang="ts">
definePageMeta({ title: 'Monitoring' })
const api = useApi()
const data = ref<any>(null)
const loading = ref(true)

async function load() {
  loading.value = true
  try { data.value = await api.get('/api/overview') } finally { loading.value = false }
}
onMounted(load)
const pct = (n: number) => `${(n ?? 0).toFixed(1)}%`
</script>

<template>
  <p class="page-desc">Live platform metrics. Each service also exposes Prometheus metrics scraped into Grafana.</p>
  <div class="toolbar"><div class="spacer" /><button class="btn ghost" @click="load">Refresh</button></div>

  <div v-if="loading" class="loading"><span class="spin" /> Loading…</div>
  <div v-else-if="data">
    <div class="grid cols-4" style="margin-bottom:16px">
      <StatCard label="Requests Today" :value="data.requests_today" />
      <StatCard label="Avg Latency" :value="`${Math.round(data.avg_latency_ms || 0)} ms`" />
      <StatCard label="Error Rate (24h)" :value="pct(data.error_rate_today)" />
      <StatCard label="Cache Hit Ratio" :value="pct(data.cache_hit_ratio)" />
    </div>

    <div class="grid cols-2" style="margin-bottom:16px">
      <div class="card">
        <div class="card-head"><h3>Top Slow Endpoints</h3></div>
        <div class="table-wrap"><table class="tbl">
          <thead><tr><th>Endpoint</th><th>Avg ms</th><th>Requests</th></tr></thead>
          <tbody>
            <tr v-for="(e, i) in data.top_slow" :key="i"><td class="mono">{{ e.label }}</td><td>{{ Math.round(e.avg_ms) }}</td><td>{{ e.count }}</td></tr>
            <tr v-if="!data.top_slow?.length"><td colspan="3" class="faint">No traffic yet</td></tr>
          </tbody>
        </table></div>
      </div>
      <div class="card">
        <div class="card-head"><h3>Top Error Endpoints</h3></div>
        <div class="table-wrap"><table class="tbl">
          <thead><tr><th>Endpoint</th><th>Error %</th><th>Requests</th></tr></thead>
          <tbody>
            <tr v-for="(e, i) in data.top_errors" :key="i"><td class="mono">{{ e.label }}</td><td>{{ e.error_rate.toFixed(1) }}%</td><td>{{ e.count }}</td></tr>
            <tr v-if="!data.top_errors?.length"><td colspan="3" class="faint">No errors</td></tr>
          </tbody>
        </table></div>
      </div>
    </div>

    <div class="grid cols-2">
      <div class="card">
        <div class="card-head"><h3>Connector Health</h3></div>
        <div class="table-wrap"><table class="tbl">
          <thead><tr><th>Connection</th><th>Type</th><th>Status</th><th>Health</th></tr></thead>
          <tbody>
            <tr v-for="(c, i) in data.connectors" :key="i">
              <td>{{ c.name }}</td><td><span class="badge blue">{{ c.database_type }}</span></td>
              <td><StatusBadge :status="c.status" /></td><td><StatusBadge :status="c.health_status" /></td>
            </tr>
            <tr v-if="!data.connectors?.length"><td colspan="4" class="faint">No connections</td></tr>
          </tbody>
        </table></div>
      </div>
      <div class="card">
        <div class="card-head"><h3>Observability Endpoints</h3></div>
        <div class="card-body">
          <div class="kv">
            <div class="k">Metrics</div><div><span class="mono">GET /metrics</span> on every service (Prometheus)</div>
            <div class="k">Liveness</div><div><span class="mono">GET /healthz</span></div>
            <div class="k">Readiness</div><div><span class="mono">GET /readyz</span></div>
            <div class="k">Dashboards</div><div>Grafana templates: Overview, Traffic, Latency, Errors, Cache, Connectors, Security</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
