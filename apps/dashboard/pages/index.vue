<script setup lang="ts">
const api = useApi()
const data = ref<any>(null)
const loading = ref(true)

async function load() {
  loading.value = true
  try { data.value = await api.get('/api/overview') } finally { loading.value = false }
}
onMounted(load)

const pct = (n: number) => `${(n ?? 0).toFixed(1)}%`
const maxTraffic = computed(() => Math.max(1, ...(data.value?.top_traffic || []).map((e: any) => e.count || 0)))
const trafficWidth = (count: number) => `${Math.max(4, Math.round(((count || 0) / maxTraffic.value) * 100))}%`
const healthLabel = computed(() => {
  const connectors = data.value?.connectors || []
  const unhealthy = connectors.filter((c: any) => !['healthy', 'active', 'closed'].includes(String(c.health_status || '').toLowerCase()))
  return unhealthy.length ? `${unhealthy.length} connector needs attention` : 'All connectors operational'
})
</script>

<template>
  <div v-if="loading" class="loading"><span class="spin" /> Loading overview…</div>
  <div v-else-if="data">
    <div class="page-intro">
      <div>
        <div class="eyebrow">CONTROL PLANE</div>
        <h2>Platform overview</h2>
        <p class="page-desc">A real-time view of gateway activity, performance, and connector health.</p>
      </div>
      <button class="btn ghost refresh-btn" @click="load"><span>↻</span> Refresh</button>
    </div>
    <section class="overview-hero" aria-label="DDAG control plane status">
      <div class="overview-hero-copy">
        <div class="eyebrow"><span class="live-dot" /> LIVE CONTROL PLANE</div>
        <h3>Ship governed APIs,<br><em>not deployment tickets.</em></h3>
        <p>Build secure database APIs, observe every request, and keep your data plane under control from one workspace.</p>
        <div class="overview-hero-actions">
          <RouterLink class="btn primary" to="/apis"><Icon name="apis" /> Build an API</RouterLink>
          <RouterLink class="btn ghost" to="/monitoring"><Icon name="monitoring" /> View telemetry</RouterLink>
        </div>
      </div>
      <div class="overview-hero-orbit" aria-hidden="true">
        <div class="orbit orbit-outer" />
        <div class="orbit orbit-inner" />
        <div class="orbit-core"><Icon name="overview" :size="30" /></div>
        <span class="orbit-node node-api">API</span><span class="orbit-node node-db">DB</span><span class="orbit-node node-auth">AUTH</span>
      </div>
      <div class="overview-status"><span class="status-dot" /><div><strong>{{ healthLabel }}</strong><small>Continuously observed</small></div></div>
    </section>

    <section class="overview-stat-grid" aria-label="Platform summary">
      <StatCard label="Active APIs" :value="data.total_apis_active" sub="Published and available" tone="blue" />
      <StatCard label="Active Clients" :value="data.total_clients_active" sub="OAuth consumers enabled" tone="violet" />
      <StatCard label="Requests Today" :value="data.requests_today" sub="Gateway traffic today" tone="green" />
      <StatCard label="Connections" :value="data.total_connections" sub="Configured source systems" tone="amber" />
    </section>
    <section class="overview-signal-grid" aria-label="Reliability signals">
      <StatCard label="Error Rate (24h)" :value="pct(data.error_rate_today)" sub="Real traffic · gateway / upstream 5xx" tone="red" />
      <StatCard label="Avg Latency" :value="`${Math.round(data.avg_latency_ms || 0)} ms`" sub="Platform responsiveness" tone="blue" />
      <StatCard label="Cache Hit Ratio" :value="pct(data.cache_hit_ratio)" sub="Responses served from cache" tone="green" />
    </section>

    <div class="grid cols-2" style="margin-bottom:16px">
      <div class="card executive-card executive-card-traffic">
        <div class="card-head"><h3>Top Endpoints by Traffic</h3></div>
        <div class="table-wrap">
          <table class="tbl">
            <thead><tr><th>Endpoint</th><th>Requests</th><th>Avg ms</th></tr></thead>
            <tbody>
              <tr v-for="(e, i) in data.top_traffic" :key="i">
                <td class="mono">{{ e.label }}</td><td>{{ e.count }}</td><td>{{ Math.round(e.avg_ms) }}</td>
              </tr>
              <tr v-if="!data.top_traffic?.length"><td colspan="3" class="faint">No traffic yet</td></tr>
            </tbody>
          </table>
        </div>
      </div>
      <div class="card executive-card executive-card-slow">
        <div class="card-head"><h3>Top Slow Endpoints</h3></div>
        <div class="table-wrap">
          <table class="tbl">
            <thead><tr><th>Endpoint</th><th>Avg ms</th><th>Requests</th></tr></thead>
            <tbody>
              <tr v-for="(e, i) in data.top_slow" :key="i">
                <td class="mono">{{ e.label }}</td><td>{{ Math.round(e.avg_ms) }}</td><td>{{ e.count }}</td>
              </tr>
              <tr v-if="!data.top_slow?.length"><td colspan="3" class="faint">No traffic yet</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div class="grid cols-2">
      <div class="card executive-card executive-card-risk">
        <div class="card-head"><h3>Top Error Endpoints</h3></div>
        <div class="table-wrap">
          <table class="tbl">
            <thead><tr><th>Endpoint</th><th>Error %</th><th>Requests</th></tr></thead>
            <tbody>
              <tr v-for="(e, i) in data.top_errors" :key="i">
                <td class="mono">{{ e.label }}</td><td>{{ e.error_rate.toFixed(1) }}%</td><td>{{ e.count }}</td>
              </tr>
              <tr v-if="!data.top_errors?.length"><td colspan="3" class="faint">No errors 🎉</td></tr>
            </tbody>
          </table>
        </div>
      </div>
      <div class="card executive-card executive-card-health">
        <div class="card-head"><h3>Connector Health</h3></div>
        <div class="table-wrap">
          <table class="tbl">
            <thead><tr><th>Connection</th><th>Type</th><th>Status</th><th>Health</th></tr></thead>
            <tbody>
              <tr v-for="(c, i) in data.connectors" :key="i">
                <td>{{ c.name }}</td>
                <td><span class="badge blue">{{ c.database_type }}</span></td>
                <td><StatusBadge :status="c.status" /></td>
                <td><StatusBadge :status="c.health_status" /></td>
              </tr>
              <tr v-if="!data.connectors?.length"><td colspan="4" class="faint">No connections</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
