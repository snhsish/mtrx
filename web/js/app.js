function j(u){return fetch(u).then(r=>r.json())}
function qs(o){const p=new URLSearchParams();for(const[k,v]of Object.entries(o))if(v)p.set(k,v);return p.toString()?'?'+p.toString():''}
function rangeDates(v){const n=new Date();let f=null,t=n;if(v==='1m')f=new Date(n);else if(v==='3m')f=new Date(n);else if(v==='6m')f=new Date(n);else if(v==='1y')f=new Date(n);if(v==='1m')f.setMonth(f.getMonth()-1);if(v==='3m')f.setMonth(f.getMonth()-3);if(v==='6m')f.setMonth(f.getMonth()-6);if(v==='1y')f.setFullYear(f.getFullYear()-1);if(v==='custom'){const cf=document.getElementById('customFrom').value,ct=document.getElementById('customTo').value;return{from:cf?new Date(cf).toISOString():'',to:ct?new Date(new Date(ct).getTime()+86400000-1).toISOString():new Date().toISOString()}}if(!f)return{from:'',to:''};return{from:f.toISOString(),to:t.toISOString()}}
function currentFilters(){const r=document.getElementById('range').value,d=rangeDates(r);return{from:d.from,to:d.to,agent:document.getElementById('agent').value,model:document.getElementById('model').value}}
function fmt(n){return Number(n).toLocaleString()}
function fmtCompact(n){n=Number(n)||0;if(n>=1e9){const v=n/1e9;return(v>=10?v.toFixed(0):v.toFixed(1).replace(/\.0$/,''))+'B'}if(n>=1e6){const v=n/1e6;return(v>=10?v.toFixed(0):v.toFixed(1).replace(/\.0$/,''))+'M'}if(n>=1e3){const v=n/1e3;return(v>=10?v.toFixed(0):v.toFixed(1).replace(/\.0$/,''))+'K'}return fmt(n)}
function fmtCost(n){n=Number(n)||0;if(n===0)return '-';return '$'+n.toFixed(n>=10?2:n>=1?3:4).replace(/0+$/,'').replace(/\.$/,'')}
function formatModel(raw){if(!raw)return 'unknown';let s=String(raw).trim();if((s.startsWith('{')&&s.endsWith('}'))||(s.startsWith('"')&&s.includes('{'))){try{const j=JSON.parse(s);if(typeof j==='string')return j;if(j.provider&&j.model)return j.provider+'/'+j.model;if(j.provider&&j.id)return j.provider+'/'+j.id;if(j.provider&&j.name)return j.provider+'/'+j.name;if(j.id)return j.id;if(j.model)return j.model;if(j.name)return j.name;if(j.provider)return j.provider;}catch{}}try{const j=JSON.parse(s);if(j&&typeof j==='object'){if(j.provider&&j.model)return j.provider+'/'+j.model;if(j.provider&&j.id)return j.provider+'/'+j.id;if(j.id)return j.id;if(j.model)return j.model}}catch{}return s}
function formatModelDisplay(raw){const d=formatModel(raw);if(d.includes('/')){const i=d.indexOf('/');const prov=d.slice(0,i);const name=d.slice(i+1);return '<span class="model-provider">'+prov+'</span><span class="model-name">'+name+'</span>'}return '<span class="model-name">'+d+'</span>'}
let modelData=[];let sortKey='total';let sortDir='desc';
function sortModels(){modelData.sort((a,b)=>{let va,vb;if(sortKey==='cost'){va=a.cost||0;vb=b.cost||0}else if(sortKey==='total'){va=(a.input||0)+(a.output||0);vb=(b.input||0)+(b.output||0)}else if(sortKey==='input'){va=a.input||0;vb=b.input||0}else if(sortKey==='output'){va=a.output||0;vb=b.output||0}else if(sortKey==='calls'){va=a.calls||0;vb=b.calls||0}else if(sortKey==='model'){va=formatModel(a.model).toLowerCase();vb=formatModel(b.model).toLowerCase();return sortDir==='asc'?va.localeCompare(vb):vb.localeCompare(va)}else{va=(a.input||0)+(a.output||0);vb=(b.input||0)+(b.output||0)}return sortDir==='asc'?va-vb:vb-va})}
function updateSortHeaders(){document.querySelectorAll('th[data-sort]').forEach(th=>{const k=th.dataset.sort;th.classList.toggle('active',k===sortKey);let arr=th.querySelector('.arrow');if(!arr){arr=document.createElement('span');arr.className='arrow';th.appendChild(arr)}arr.textContent=k===sortKey?(sortDir==='asc'?'▲':'▼'):'↕';arr.style.opacity=k===sortKey?'1':'.35'})}
function renderModels(){sortModels();updateSortHeaders();const hasCost=modelData.some(x=>x.cost&&x.cost>0);const tb=document.getElementById('models');if(!modelData.length){tb.innerHTML='<tr><td colspan=6 class="empty">no model usage yet</td></tr>';return}tb.innerHTML=modelData.map(x=>{const tot=(x.input||0)+(x.output||0);const disp=formatModelDisplay(x.model);const raw=String(x.model).replace(/"/g,'&quot;');return `<tr><td title="${raw}">${disp}</td><td>${fmt(x.calls)}</td><td title="${fmt(x.input)}">${fmtCompact(x.input)}<br><span class="muted-sm">${fmt(x.input)}</span></td><td title="${fmt(x.output)}">${fmtCompact(x.output)}<br><span class="muted-sm">${fmt(x.output)}</span></td><td title="${fmt(tot)}"><b>${fmtCompact(tot)}</b><br><span class="muted-sm">${fmt(tot)}</span></td><td class="cost">${x.cost?fmtCost(x.cost):'-'}</td></tr>`}).join('')}
function loadAll(){
 const f=currentFilters(),q=qs(f);
 j('/api/v1/metrics/tokens'+q).then(d=>{
  const el=document.getElementById('tokens');
  const costEl=document.getElementById('cost');
  if(!d||!d.length){el.innerHTML='<div class="empty">No token usage in this period</div>';if(costEl)costEl.innerHTML='<div class="empty">No cost data</div>';return}
  const tot=d.reduce((a,x)=>({input:a.input+x.input,output:a.output+x.output,total:a.total+x.total,count:a.count+x.count,cost:(a.cost||0)+(x.cost||0)}),{input:0,output:0,total:0,count:0,cost:0});
  el.innerHTML=`<div class="stats-grid">
   <div class="stat"><span class="stat-value" title="${fmt(tot.total)}">${fmtCompact(tot.total)}</span><span class="stat-label">Total tokens</span><span class="stat-sub">${fmt(tot.count)} sessions</span></div>
   <div class="stat"><span class="stat-value" title="${fmt(tot.input)}">${fmtCompact(tot.input)}</span><span class="stat-label">Input</span><span class="stat-sub" title="${fmt(tot.input)}">${fmt(tot.input)}</span></div>
   <div class="stat"><span class="stat-value" title="${fmt(tot.output)}">${fmtCompact(tot.output)}</span><span class="stat-label">Output</span><span class="stat-sub" title="${fmt(tot.output)}">${fmt(tot.output)}</span></div>
   <div class="stat stat-cost"><span class="stat-value">${tot.cost?fmtCost(tot.cost):'-'}</span><span class="stat-label">Total cost</span><span class="stat-sub">${tot.cost?'avg '+fmtCost(tot.cost/Math.max(1,tot.total/1e6))+'/1M':tot.total?fmtCompact(tot.total)+' tokens':'no data'}</span></div>
  </div>`;
  if(costEl){
   if(tot.cost){costEl.innerHTML=`<div class="stats-grid">
    <div class="stat stat-cost"><span class="stat-value">${fmtCost(tot.cost)}</span><span class="stat-label">Total</span><span class="stat-sub">${fmtCompact(tot.total)} tokens</span></div>
    <div class="stat"><span class="stat-value">${fmtCost(tot.cost/Math.max(1,tot.total/1e6))}</span><span class="stat-label">Avg / 1M tokens</span><span class="stat-sub">${fmt(tot.count)} sessions</span></div>
    <div class="stat"><span class="stat-value">${fmtCost(tot.cost/Math.max(1,tot.count))}</span><span class="stat-label">Avg / session</span><span class="stat-sub">${fmtCompact(tot.total)} total</span></div>
    <div class="stat"><span class="stat-value">${fmtCompact(tot.total)}</span><span class="stat-label">Tokens</span><span class="stat-sub">${fmt(tot.input)} in · ${fmt(tot.output)} out</span></div>
   </div>`}else{costEl.innerHTML='<div class="empty">No cost data for this period</div>'}
  }
 }).catch(()=>{})
 j('/api/v1/metrics/models'+q).then(d=>{modelData=d||[];renderModels()}).catch(()=>{})
}
function populateFilters(){
 j('/api/v1/meta/agents').then(a=>{const s=document.getElementById('agent');const cur=s.value;s.innerHTML='<option value="">All harnesses</option>';a.forEach(v=>{const o=document.createElement('option');o.value=v;o.textContent=v;s.appendChild(o)});if(cur&&[...s.options].some(o=>o.value===cur))s.value=cur}).catch(()=>{});
 j('/api/v1/meta/models').then(a=>{const s=document.getElementById('model');const cur=s.value;s.innerHTML='<option value="">All models</option>';a.forEach(v=>{const o=document.createElement('option');o.value=v;o.textContent=formatModel(v);s.appendChild(o)});if(cur&&[...s.options].some(o=>o.value===cur))s.value=cur}).catch(()=>{})
}
document.getElementById('range').addEventListener('change',e=>{const c=e.target.value==='custom';document.getElementById('customFromWrap').style.display=c?'':'none';document.getElementById('customToWrap').style.display=c?'':'none';if(!c)loadAll()});
document.getElementById('customFrom').addEventListener('change',()=>{if(document.getElementById('range').value==='custom')loadAll()});
document.getElementById('customTo').addEventListener('change',()=>{if(document.getElementById('range').value==='custom')loadAll()});
document.getElementById('agent').addEventListener('change',loadAll);
document.getElementById('model').addEventListener('change',loadAll);
document.getElementById('reset').addEventListener('click',()=>{document.getElementById('range').value='1y';document.getElementById('agent').value='';document.getElementById('model').value='';document.getElementById('customFrom').value='';document.getElementById('customTo').value='';document.getElementById('customFromWrap').style.display='none';document.getElementById('customToWrap').style.display='none';loadAll()});
document.querySelectorAll('th[data-sort]').forEach(th=>{th.addEventListener('click',()=>{const k=th.dataset.sort;if(sortKey===k)sortDir=sortDir==='asc'?'desc':'asc';else{sortKey=k;sortDir=k==='model'||k==='calls'?'asc':'desc'}renderModels()})});
populateFilters();loadAll();
try{var es=new EventSource('/api/v1/live');es.onopen=()=>{const d=document.querySelector('#live i');if(d)d.style.background='#22c55e'};es.onerror=()=>{const d=document.querySelector('#live i');if(d)d.style.background='#71717a'}}catch(e){}
