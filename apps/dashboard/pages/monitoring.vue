<script setup lang="ts">
definePageMeta({ title: 'Monitoring' })
const api = useApi()
const data = ref<any>(null)
const circuits = ref<any[]>([])
const pools = ref<any[]>([])
const loading = ref(true)

async function load() {
  loading.value = true
  try {
    const [overview, circuitRows, poolRows] = await Promise.all([
      api.get('/api/overview'),
      api.get('/api/circuit-breakers').catch(() => []),
      api.get('/api/pool-stats').catch(() => []),
    ])
    data.value = overview
    circuits.value = circuitRows || []
    pools.value = poolRows || []
  } finally { loading.value = false }
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
          <thead><tr><th>Connection</th><th>Type</th><th>Status</th><th>Health</th><th>Circuit</th></tr></thead>
          <tbody>
            <tr v-for="(c, i) in data.connectors" :key="i">
              <td>{{ c.name }}</td><td><span class="badge blue">{{ c.database_type }}</span></td>
              <td><StatusBadge :status="c.status" /></td><td><StatusBadge :status="c.health_status" /></td>
              <td><StatusBadge :status="circuits.find((x) => x.connection_id === c.id)?.state || 'closed'" /></td>
            </tr>
            <tr v-if="!data.connectors?.length"><td colspan="5" class="faint">No connections</td></tr>
          </tbody>
        </table></div>
      </div>
      <div class="card">
        <div class="card-head"><h3>Pool Usage</h3></div>
        <div class="table-wrap"><table class="tbl">
          <thead><tr><th>Connection</th><th>Active</th><th>Idle</th><th>Max</th><th>Wait</th><th>Timeout</th></tr></thead>
          <tbody>
            <tr v-for="p in pools" :key="p.connection_id">
              <td>{{ p.connection }}</td><td>{{ p.in_use }}</td><td>{{ p.idle }}</td><td>{{ p.max }}</td><td>{{ p.wait_count }}</td><td>{{ p.timeout_count }}</td>
            </tr>
            <tr v-if="!pools.length"><td colspan="6" class="faint">No active pools</td></tr>
          </tbody>
        </table></div>
      </div>
    </div>
  </div>
</template>
