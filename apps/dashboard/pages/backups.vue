<script setup lang="ts">
type Destination = { id:string; name:string; provider:string; config:Record<string,any>; has_credential:boolean; status:string; last_error?:string; last_verified_at?:string }
type Provider = 'local'|'s3'|'b2'|'gdrive'|'sftp'
type BackupRun = { id:string; kind:string; status:string; artifact_path?:string; bytes?:number; started_at:string; completed_at?:string; error?:string }
const api = useApi(); const toast = useToast()
const loading=ref(true), saving=ref(false), verifying=ref(''), running=ref(''), error=ref(''), editorOpen=ref(false)
const destinations=ref<Destination[]>([])
const runs=ref<BackupRun[]>([])
const editor=reactive({name:'',provider:'local' as Provider,path:'backups',endpoint:'',bucket:'',region:'auto',prefix:'ddag',host:'',port:22,remotePath:'/backups',accessKey:'',secretKey:'',applicationKey:'',oauthJson:'',username:'',password:'',privateKey:''})
const providers:{value:Provider;label:string;description:string;credential:string}[]=[
 {value:'local',label:'Local encrypted',description:'Store encrypted artifacts on the DDAG host. Good for a first verified copy, not offsite protection.',credential:'No credential needed.'},
 {value:'s3',label:'S3-compatible',description:'AWS S3, Cloudflare R2, MinIO, Wasabi, and compatible object storage.',credential:'Access key and secret are sealed with envelope encryption.'},
 {value:'b2',label:'Backblaze B2',description:'Offsite object storage for economical, durable retention.',credential:'Application key data is write-only and encrypted.'},
 {value:'gdrive',label:'Google Drive',description:'Use a dedicated backup folder and OAuth/service credential.',credential:'OAuth payload is write-only and encrypted.'},
 {value:'sftp',label:'SFTP / SSH',description:'Upload encrypted artifacts to an SSH/SFTP server.',credential:'Password or private key is write-only and encrypted.'},
]
const selectedProvider=computed(()=>providers.find(p=>p.value===editor.provider)!)
const verified=computed(()=>destinations.value.filter(d=>d.status==='verified'||d.status==='enabled'))
const statusLabel=(d:Destination)=>d.status==='verified'||d.status==='enabled'?'Ready':d.status==='failed'?'Needs attention':'Not verified'
function reset(){Object.assign(editor,{name:'',provider:'local',path:'backups',endpoint:'',bucket:'',region:'auto',prefix:'ddag',host:'',port:22,remotePath:'/backups',accessKey:'',secretKey:'',applicationKey:'',oauthJson:'',username:'',password:'',privateKey:''})}
function openEditor(){reset();editorOpen.value=true}
function providerConfig(){switch(editor.provider){case 'local':return {path:editor.path};case 's3':case 'b2':return {endpoint:editor.endpoint,bucket:editor.bucket,region:editor.region,prefix:editor.prefix};case 'gdrive':return {folder:editor.remotePath};case 'sftp':return {host:editor.host,port:Number(editor.port),path:editor.remotePath,username:editor.username}}}
function providerCredential(){switch(editor.provider){case 's3':return {access_key_id:editor.accessKey,secret_access_key:editor.secretKey};case 'b2':return {application_key:editor.applicationKey};case 'gdrive':try{return JSON.parse(editor.oauthJson)}catch{return null};case 'sftp':return editor.privateKey?{private_key:editor.privateKey}:{password:editor.password};default:return undefined}}
function valid(){if(!editor.name.trim())return false;if(editor.provider==='local')return !!editor.path.trim();if(editor.provider==='s3'||editor.provider==='b2')return !!(editor.endpoint&&editor.bucket&&editor.accessKey&&editor.secretKey);if(editor.provider==='gdrive')return !!editor.oauthJson;return !!(editor.host&&editor.username&&(editor.password||editor.privateKey))}
const checked = (value: string | null | undefined) => value ? new Date(value).toLocaleString() : 'Never'
const duration = (start?: string, end?: string) => {
  if (!start || !end) return ''
  const s = Math.round((new Date(end).getTime() - new Date(start).getTime()) / 1000)
  return s > 60 ? `${Math.floor(s/60)}m ${s%60}s` : `${s >= 0 ? s : 0}s`
}
const latest=(kind:string)=>runs.value.find(r=>r.kind===kind)
const backupReady=computed(()=>latest('logical_backup')?.status==='succeeded')
async function load(){loading.value=true;error.value='';try{const [d,r]=await Promise.all([api.get<Destination[]>('/api/backup-destinations'),api.get<BackupRun[]>('/api/backup-runs')]);destinations.value=d||[];runs.value=r||[]}catch(e:any){error.value=e.message||'Unable to load backup recovery status'}finally{loading.value=false}}
async function trigger(kind:'logical-backup'|'restore-drill'){running.value=kind;try{await api.post(`/api/backup-runs/${kind}`,{});toast.success(kind==='logical-backup'?'Backup started':'Restore drill started','The operation is running in the protected backend. Refresh shortly for evidence.');setTimeout(load,1800)}catch(e:any){toast.error('Could not start recovery operation',e.message)}finally{running.value=''}}
async function createDestination(){const credential=providerCredential();if(editor.provider==='gdrive'&&!credential){toast.error('Invalid credential','Paste a valid OAuth/service-account JSON object.');return}saving.value=true;try{await api.post('/api/backup-destinations',{name:editor.name.trim(),provider:editor.provider,config:providerConfig(),credential,enabled:true});toast.success('Destination created','Credentials are encrypted and cannot be read back. Test it before it is eligible for backup.');editorOpen.value=false;await load()}catch(e:any){toast.error('Could not create destination',e.message)}finally{saving.value=false}}
async function verify(d:Destination){verifying.value=d.id;try{await api.post(`/api/backup-destinations/${d.id}/verify`);toast.success('Destination verified',`${d.name} is now eligible for the backup runner.`)}catch(e:any){toast.error('Verification failed',e.message)}finally{verifying.value='';await load()}}
onMounted(load)
</script>
<template>
<PageHeader eyebrow="RESILIENCE" title="Backup & Recovery" icon="database" description="Prepare encrypted storage destinations and verify recovery readiness without exposing credentials.">
 <template #actions><button class="btn" :disabled="loading" @click="load">Refresh</button><button class="btn primary" @click="openEditor">Add destination</button></template>
</PageHeader>
<div v-if="error" class="alert danger"><b>Backup destinations unavailable.</b> {{ error }} <button class="btn sm" @click="load">Try again</button></div>
<section class="readiness card">
 <div class="readiness-copy"><div class="eyebrow">RECOVERY POSTURE</div><h3>Build a verified recovery chain</h3><p>Credentials are sealed at rest. A destination must pass verification before a future backup runner can use it.</p></div>
 <div class="readiness-steps"><div class="step done"><b>1</b><span>Destination</span><small>{{ destinations.length }} configured</small></div><div :class="['step',verified.length?'done':'']"><b>2</b><span>Verification</span><small>{{ verified.length }} ready</small></div><div class="step done"><b>3</b><span>Automation</span><small>Host scheduler managed</small></div></div>
</section>
<section class="card destinations-card">
 <div class="card-top"><div><div class="eyebrow">STORAGE TARGETS</div><h3>Backup destinations</h3><p class="faint">Add a local copy or an offsite provider. Secret fields are encrypted before storage and never returned to this browser.</p></div><span class="badge blue">Provider-neutral</span></div>
 <div v-if="loading" class="loading"><i class="spin"/> Loading destinations…</div>
 <div v-else-if="!destinations.length" class="empty"><div class="state-orb">+</div><b>No backup destination yet</b><span>Create and test a destination before enabling any backup automation.</span><button class="btn primary" @click="openEditor">Add first destination</button></div>
 <div v-else class="destination-list"><article v-for="d in destinations" :key="d.id" class="destination-row"><div class="provider-icon">{{ d.provider==='local'?'◫':d.provider==='sftp'?'↗':'◌' }}</div><div class="destination-main"><b>{{d.name}}</b><div class="destination-meta"><span>{{d.provider==='s3'?'S3-compatible':d.provider}}</span><span>{{d.has_credential?'Credential sealed':'No credential required'}}</span><span v-if="d.last_verified_at">Verified {{new Date(d.last_verified_at).toLocaleDateString()}}</span></div><small v-if="d.last_error" class="error-text">{{d.last_error}}</small></div><div class="destination-state"><span :class="['badge',d.status==='verified'||d.status==='enabled'?'green':d.status==='failed'?'red':'amber']">{{statusLabel(d)}}</span><button class="btn sm" :disabled="verifying===d.id" @click="verify(d)">{{verifying===d.id?'Testing…':d.status==='verified'||d.status==='enabled'?'Re-test':'Test connection'}}</button></div></article></div>
</section>
<section class="runner-grid">
 <article class="card runner-card"><div class="runner-head"><div class="runner-icon">◷</div><span :class="['badge',backupReady?'green':'amber']">{{backupReady?'Verified':'Ready to run'}}</span></div><h3>Encrypted logical backup</h3><p>Create AES-256-GCM encrypted PostgreSQL dumps with SHA-256 evidence and 30-day local retention.</p><div class="runner-line"><span>Status</span><b>{{latest('logical_backup')?.status||'Not run'}}</b><small v-if="latest('logical_backup')?.completed_at || latest('logical_backup')?.started_at" class="faint" style="display:block;margin-top:2px">{{ checked(latest('logical_backup')?.completed_at || latest('logical_backup')?.started_at) }} <template v-if="duration(latest('logical_backup')?.started_at, latest('logical_backup')?.completed_at)">({{ duration(latest('logical_backup')?.started_at, latest('logical_backup')?.completed_at) }})</template></small></div><div class="runner-line"><span>Artifact</span><b class="mono" style="word-break:break-all">{{latest('logical_backup')?.artifact_path ? latest('logical_backup')?.artifact_path?.split('/').pop() : '—'}}</b><small v-if="latest('logical_backup')?.bytes" class="faint" style="display:block;margin-top:2px">{{ Math.round(latest('logical_backup')!.bytes!/1024) }} KB</small></div><button class="btn primary" :disabled="!!running" @click="trigger('logical-backup')">{{running==='logical-backup'?'Starting…':'Run encrypted backup now'}}</button><div v-if="latest('logical_backup')?.error" class="error-text">{{latest('logical_backup')?.error}}</div></article>
 <article class="card runner-card"><div class="runner-head"><div class="runner-icon">✓</div><span :class="['badge',latest('restore_drill')?.status==='succeeded'?'green':'gray']">{{latest('restore_drill')?.status==='succeeded'?'Verified':'Awaiting backup'}}</span></div><h3>Isolated restore drill</h3><p>Decrypt the latest artifact, restore into a temporary database, then validate application table presence.</p><div class="runner-line"><span>Status</span><b>{{latest('restore_drill')?.status||'Not run'}}</b><small v-if="latest('restore_drill')?.completed_at || latest('restore_drill')?.started_at" class="faint" style="display:block;margin-top:2px">{{ checked(latest('restore_drill')?.completed_at || latest('restore_drill')?.started_at) }} <template v-if="duration(latest('restore_drill')?.started_at, latest('restore_drill')?.completed_at)">({{ duration(latest('restore_drill')?.started_at, latest('restore_drill')?.completed_at) }})</template></small></div><div class="runner-line"><span>Target Artifact</span><b class="mono" style="word-break:break-all">{{latest('restore_drill')?.artifact_path ? latest('restore_drill')?.artifact_path?.split('/').pop() : '—'}}</b><small class="faint" style="display:block;margin-top:2px">Isolated verification in temp schema</small></div><button class="btn" :disabled="!backupReady||!!running" @click="trigger('restore-drill')">{{running==='restore-drill'?'Starting…':'Run restore drill'}}</button><div v-if="latest('restore_drill')?.error" class="error-text">{{latest('restore_drill')?.error}}</div></article>
</section>
<UiModal :open="editorOpen" title="Add encrypted destination" wide @close="editorOpen=false"><div class="destination-form"><div class="form-intro"><h3>{{selectedProvider.label}}</h3><p>{{selectedProvider.description}}</p></div><div class="provider-picker"><button v-for="p in providers" :key="p.value" :class="{selected:editor.provider===p.value}" type="button" @click="editor.provider=p.value">{{p.label}}</button></div><div class="field"><label>Destination name</label><input v-model="editor.name" placeholder="e.g. production-r2" autocomplete="off"/></div><template v-if="editor.provider==='local'"><div class="field"><label>Storage path</label><input v-model="editor.path" placeholder="backups"/><small class="hint">Path relative to DDAG’s managed backup storage.</small></div></template><template v-else-if="editor.provider==='s3'||editor.provider==='b2'"><div class="form-grid"><div class="field"><label>Endpoint</label><input v-model="editor.endpoint" placeholder="https://account.r2.cloudflarestorage.com"/></div><div class="field"><label>Bucket</label><input v-model="editor.bucket" placeholder="ddag-backups"/></div><div class="field"><label>Region</label><input v-model="editor.region" placeholder="auto"/></div><div class="field"><label>Prefix</label><input v-model="editor.prefix" placeholder="ddag"/></div></div><div class="secret-panel"><b>Write-only credentials</b><p>{{selectedProvider.credential}}</p><div class="form-grid"><div class="field"><label>Access key ID</label><input v-model="editor.accessKey" autocomplete="off"/></div><div class="field"><label>Secret access key</label><input v-model="editor.secretKey" type="password" autocomplete="new-password"/></div></div></div></template><template v-else-if="editor.provider==='gdrive'"><div class="field"><label>Backup folder</label><input v-model="editor.remotePath" placeholder="DDAG Backups"/></div><div class="secret-panel field"><b>OAuth or service account JSON</b><p>{{selectedProvider.credential}}</p><textarea v-model="editor.oauthJson" rows="6" placeholder='{"client_id":"…","client_secret":"…"}'/></div></template><template v-else><div class="form-grid"><div class="field"><label>Host</label><input v-model="editor.host" placeholder="backup.example.com"/></div><div class="field"><label>Port</label><input v-model.number="editor.port" type="number" min="1" max="65535"/></div><div class="field"><label>Remote path</label><input v-model="editor.remotePath" placeholder="/srv/backups/ddag"/></div><div class="field"><label>Username</label><input v-model="editor.username" autocomplete="off"/></div></div><div class="secret-panel"><b>Write-only authentication</b><p>Provide either a password or an SSH private key. The value is encrypted and cannot be displayed after saving.</p><div class="field"><label>Password</label><input v-model="editor.password" type="password" autocomplete="new-password"/></div><div class="field"><label>SSH private key</label><textarea v-model="editor.privateKey" rows="4" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"/></div></div></template></div><template #footer><button class="btn" @click="editorOpen=false">Cancel</button><button class="btn primary" :disabled="saving||!valid()" @click="createDestination">{{saving?'Saving…':'Save encrypted destination'}}</button></template></UiModal>
</template>
<style scoped>
.alert{padding:12px 14px;border:1px solid rgba(239,68,68,.3);border-radius:14px;background:rgba(239,68,68,.09);margin-bottom:16px}.readiness{display:flex;justify-content:space-between;align-items:center;gap:24px;padding:20px;margin-bottom:16px;background:linear-gradient(115deg,rgba(87,92,240,.13),rgba(15,23,42,.45))}.readiness h3{font-size:18px;margin:5px 0}.readiness p{margin:0;color:var(--text-dim);max-width:580px}.readiness-steps{display:flex;min-width:460px}.step{flex:1;position:relative;padding-left:38px;color:var(--text-faint)}.step:before{content:"";position:absolute;left:14px;right:-14px;top:15px;height:1px;background:var(--border)}.step:last-child:before{right:50%}.step b{position:absolute;z-index:1;left:0;top:0;width:30px;height:30px;display:grid;place-items:center;border-radius:50%;background:var(--panel-2);border:1px solid var(--border)}.step.done b{background:var(--success);border-color:var(--success);color:#062b14}.step span,.step small{display:block}.step span{font-weight:800;color:var(--text);font-size:12px}.step small{font-size:11px}.card{padding:20px}.destinations-card{padding:0;overflow:hidden}.card-top{display:flex;justify-content:space-between;gap:16px;padding:20px;border-bottom:1px solid var(--border)}.card-top h3{font-size:18px;margin:5px 0}.card-top p{margin:0}.empty{display:grid;gap:10px;justify-items:center}.empty .btn{margin-top:8px}.destination-list{padding:8px}.destination-row{display:flex;align-items:center;gap:14px;padding:14px 12px;border-radius:12px}.destination-row:hover{background:rgba(109,114,255,.06)}.provider-icon,.runner-icon{display:grid;place-items:center;width:38px;height:38px;border-radius:13px;background:rgba(109,114,255,.12);border:1px solid rgba(109,114,255,.2);color:#a5b4fc;font-weight:900}.destination-main{flex:1;min-width:0}.destination-meta{display:flex;gap:9px;flex-wrap:wrap;color:var(--text-faint);font-size:11px;margin-top:4px}.destination-meta span+span:before{content:'·';margin-right:9px}.destination-state{display:flex;align-items:center;gap:10px}.runner-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:16px}.runner-card{min-height:250px}.runner-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:18px}.runner-card h3{font-size:17px}.runner-card>p{color:var(--text-dim);min-height:44px}.runner-line{display:flex;justify-content:space-between;padding:10px 0;border-top:1px solid var(--border);font-size:12px}.runner-line span{color:var(--text-faint)}.runner-foot{margin-top:14px;padding:10px;border-radius:10px;background:rgba(245,158,11,.08);color:var(--text-dim);font-size:12px}.destination-form{display:grid;gap:14px}.form-intro h3,.form-intro p{margin:0}.form-intro p{color:var(--text-dim);margin-top:4px}.provider-picker{display:grid;grid-template-columns:repeat(5,1fr);gap:7px}.provider-picker button{padding:9px 6px;border:1px solid var(--border);border-radius:9px;background:transparent;color:var(--text-dim);font-size:11px;font-weight:700;cursor:pointer}.provider-picker button.selected{border-color:var(--primary);background:rgba(109,114,255,.13);color:var(--text)}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 12px}.secret-panel{padding:14px;border-radius:12px;border:1px solid rgba(109,114,255,.22);background:rgba(109,114,255,.06)}.secret-panel p{margin:3px 0 12px;color:var(--text-faint);font-size:12px}.field{margin:0}.hint{color:var(--text-faint)}@media(max-width:1000px){.readiness{align-items:flex-start;flex-direction:column}.readiness-steps{min-width:0;width:100%}}@media(max-width:720px){.runner-grid{grid-template-columns:1fr}.card-top,.destination-row{align-items:flex-start;flex-direction:column}.destination-state{width:100%;justify-content:space-between}.provider-picker{grid-template-columns:1fr 1fr}.form-grid{grid-template-columns:1fr}.readiness-steps{gap:8px}.step{padding-left:0;padding-top:38px}.step b{left:0}.step:before{left:15px;right:-15px}.step span{font-size:11px}}
/* Backup & Recovery layout refinement — scoped to this page. */
:deep(.page-header) {
  margin-bottom: 18px;
}

.readiness {
  align-items: flex-start;
  gap: clamp(28px, 5vw, 72px);
  padding: 24px 28px;
  margin-bottom: 18px;
}

.readiness-copy {
  flex: 1 1 520px;
  max-width: 680px;
}

.readiness-copy p,
.card-top p,
.runner-card > p {
  line-height: 1.55;
}

.readiness-steps {
  flex: 0 1 520px;
  min-width: 0;
  padding-top: 14px;
}

.step {
  min-width: 0;
  padding: 38px 14px 0 0;
}

.step::before {
  left: 15px;
  right: 0;
}

.step b {
  left: 0;
}

.step span {
  font-size: 13px;
}

.step small {
  margin-top: 3px;
  line-height: 1.35;
}

.card-top {
  align-items: flex-start;
  padding: 24px 28px 22px;
}

.card-top > div {
  max-width: 820px;
}

.card-top > .badge {
  flex: 0 0 auto;
  width: auto;
  height: auto;
  min-height: 28px;
  padding: 6px 11px;
  border-radius: 999px;
  white-space: nowrap;
}

.destination-list {
  padding: 6px 14px;
}

.destination-row {
  min-height: 72px;
  padding: 14px;
}

.destination-state {
  flex: 0 0 auto;
  justify-content: flex-end;
}

.runner-grid {
  gap: 18px;
  margin-top: 18px;
  align-items: stretch;
}

.runner-card {
  min-height: 310px;
  padding: 24px 28px;
  display: flex;
  flex-direction: column;
}

.runner-card h3 {
  margin: 22px 0 8px;
}

.runner-card > p {
  min-height: 48px;
  max-width: 620px;
  margin: 0 0 20px;
}

.runner-line {
  padding: 13px 0;
}

.runner-card > .btn {
  align-self: flex-start;
  margin-top: auto;
}

@media (max-width: 1100px) {
  .readiness {
    flex-direction: column;
  }
  .readiness-copy,
  .readiness-steps {
    flex-basis: auto;
    width: 100%;
    max-width: none;
  }
}

@media (max-width: 720px) {
  .readiness,
  .card-top,
  .runner-card {
    padding: 18px;
  }
  .readiness-steps {
    display: grid;
    gap: 12px;
    padding-top: 4px;
  }
  .step {
    padding: 4px 0 4px 42px;
    min-height: 34px;
  }
  .step::before {
    left: 15px;
    right: auto;
    top: 30px;
    bottom: -14px;
    width: 1px;
    height: auto;
  }
  .step:last-child::before {
    display: none;
  }
  .card-top,
  .destination-row {
    align-items: stretch;
  }
  .card-top {
    flex-direction: column;
  }
  .card-top > .badge {
    align-self: flex-start;
  }
  .destination-row {
    flex-wrap: wrap;
  }
  .destination-main {
    flex: 1 1 calc(100% - 56px);
  }
  .destination-state {
    width: 100%;
    padding-left: 52px;
    justify-content: space-between;
  }
  .runner-card > p {
    min-height: 0;
  }
}
</style>
