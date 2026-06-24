(function(){const e=document.createElement("link").relList;if(e&&e.supports&&e.supports("modulepreload"))return;for(const n of document.querySelectorAll('link[rel="modulepreload"]'))s(n);new MutationObserver(n=>{for(const r of n)if(r.type==="childList")for(const a of r.addedNodes)a.tagName==="LINK"&&a.rel==="modulepreload"&&s(a)}).observe(document,{childList:!0,subtree:!0});function o(n){const r={};return n.integrity&&(r.integrity=n.integrity),n.referrerPolicy&&(r.referrerPolicy=n.referrerPolicy),n.crossOrigin==="use-credentials"?r.credentials="include":n.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function s(n){if(n.ep)return;n.ep=!0;const r=o(n);fetch(n.href,r)}})();class v extends Error{constructor(e,o){super(o),this.status=e,this.name="ApiError"}}async function d(t,e,o){const s={method:t,headers:{}};o!==void 0&&(s.body=JSON.stringify(o),s.headers["Content-Type"]="application/json");const n=await fetch(e,s),r=await n.text();let a;if(r.length>0)try{a=JSON.parse(r)}catch{}if(!n.ok){const i=a?.error??r??`HTTP ${n.status}`;throw new v(n.status,i)}return a}const T={listTasks:()=>d("GET","/api/tasks"),getTask:t=>d("GET",`/api/tasks/${t}`),createTask:t=>d("POST","/api/tasks",t),patchTask:(t,e)=>d("PATCH",`/api/tasks/${t}`,e),toggleTask:t=>d("POST",`/api/tasks/${t}/toggle`),deleteTask:t=>d("DELETE",`/api/tasks/${t}`),stats:()=>d("GET","/api/stats"),health:()=>d("GET","/api/health")};function l(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}function E(t,e){if(!t)return null;const[o,s,n]=t.split("-").map(f=>parseInt(f,10));if(!o||!s||!n)return t;const r=new Date(o,s-1,n),a=new Date(e.getFullYear(),e.getMonth(),e.getDate()),i=Math.round((r.getTime()-a.getTime())/864e5);return i===0?"today":i===1?"tomorrow":i===-1?"yesterday":i<0?`${-i}d ago`:i<7?`in ${i}d`:i<14?"next week":r.toLocaleDateString(void 0,{month:"short",day:"numeric"})}function b(t,e,o){if(!t||e)return"";const[s,n,r]=t.split("-").map(f=>parseInt(f,10));if(!s||!n||!r)return"";const a=new Date(s,n-1,r).getTime(),i=new Date(o.getFullYear(),o.getMonth(),o.getDate()).getTime();return a<i?"is-overdue":a===i?"is-today":""}function L(t,e){const o=b(t.due,t.done,e),s=["row",t.done?"is-done":"",o].filter(Boolean).join(" "),n=t.due?E(t.due,e):null,r=t.tags.map(a=>`<span class="tag">${l(a)}</span>`).join("");return`
    <li class="${s}" data-id="${t.id}">
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${t.done?"checked":""}>
      <div class="title-wrap">
        <span class="title" title="${l(t.title)}">${l(t.title)}</span>
        <span class="id">#${t.id}</span>
      </div>
      <div class="meta">
        ${r?`<span class="tags">${r}</span>`:""}
        ${n?`<span class="due" title="${l(t.due??"")}">${l(n)}</span>`:""}
        <span class="priority ${l(t.priority)}" title="${l(t.priority)} priority">${S(t.priority)}</span>
      </div>
    </li>`}function S(t){switch(t){case"urgent":return"U";case"high":return"H";case"low":return"L";default:return"M"}}function M(t,e){return t.length===0?`
      <div class="empty">
        <div class="glyph">✓</div>
        <div>No tasks yet.</div>
        <div class="hint">Add one from the CLI: <code>tsk add "buy milk"</code></div>
      </div>`:`<ul class="list">${[...t].sort((n,r)=>{if(n.done!==r.done)return n.done?1:-1;const a={urgent:0,high:1,medium:2,low:3},i=a[n.priority]??9,f=a[r.priority]??9;return i!==f?i-f:n.id-r.id}).map(n=>L(n,e)).join("")}</ul>`}function D(t){let e=0,o=0;for(const s of t)s.done?e++:o++;return`<strong>${o}</strong> undone &middot; <strong>${e}</strong> done &middot; <strong>${t.length}</strong> total`}const w=document.getElementById("root");if(!w)throw new Error("missing #root");w.innerHTML=`
  <div class="app" data-app>
    <header class="topbar">
      <h1>tsk<span class="dot" data-dot>// loading</span></h1>
      <div class="file" data-file>—</div>
    </header>
    <div data-content>
      <div class="skeleton" aria-busy="true" aria-label="loading tasks">
        <div class="bar w-80"></div>
        <div class="bar w-60"></div>
        <div class="bar w-40"></div>
      </div>
    </div>
    <footer class="statusline">
      <span class="count" data-count></span>
      <span data-build>tsk web &middot; <a href="/api/tasks" style="color:inherit">api</a></span>
    </footer>
  </div>
`;const u={content:p("[data-content]"),file:p("[data-file]"),dot:p("[data-dot]"),count:p("[data-count]")};function p(t){const e=document.querySelector(t);if(!e)throw new Error(`missing ${t}`);return e}let c=[];const y=new Set;function g(){const t=new Date;u.content.innerHTML=M(c,t),u.count.innerHTML=D(c)}async function h(){try{const{file:t,tasks:e}=await T.listTasks();c=e,u.file.textContent=t,m("ready",!1),g()}catch(t){N(t)}}function m(t,e){u.dot.textContent=`// ${t}`,u.dot.style.color=e?"var(--color-prio-urgent)":"var(--color-text-faint)"}function N(t){const e=$(t);m("offline",!0),u.content.innerHTML=`
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${x(e)}</code>
    </div>`}function $(t){return t instanceof v?`${t.status}: ${t.message}`:t instanceof Error?t.message:String(t)}function x(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}async function k(t){if(y.has(t))return;const e=c.findIndex(s=>s.id===t);if(e<0)return;const o=c[e];c[e]={...o,done:!o.done},y.add(t),g();try{const s=await T.toggleTask(t);c[e]=s,g()}catch(s){c[e]=o,g(),m(`toggle failed: ${$(s)}`,!0),setTimeout(()=>m("ready",!1),3e3)}finally{y.delete(t)}}u.content.addEventListener("change",t=>{const e=t.target;if(!e||!(e instanceof HTMLInputElement)||e.dataset.toggle===void 0)return;const o=e.closest("[data-id]");if(!o)return;const s=Number(o.dataset.id);!Number.isFinite(s)||s<=0||k(s)});document.addEventListener("visibilitychange",()=>{document.visibilityState==="visible"&&h()});document.addEventListener("keydown",t=>{if(t.key!=="r"||t.metaKey||t.ctrlKey||t.altKey)return;const e=t.target;e&&(e.tagName==="INPUT"||e.tagName==="TEXTAREA"||e.isContentEditable)||(t.preventDefault(),h())});window.tsk={refresh:h,tasks:()=>c,toggle:k};h();
