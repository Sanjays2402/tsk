(function(){const s=document.createElement("link").relList;if(s&&s.supports&&s.supports("modulepreload"))return;for(const e of document.querySelectorAll('link[rel="modulepreload"]'))i(e);new MutationObserver(e=>{for(const o of e)if(o.type==="childList")for(const n of o.addedNodes)n.tagName==="LINK"&&n.rel==="modulepreload"&&i(n)}).observe(document,{childList:!0,subtree:!0});function a(e){const o={};return e.integrity&&(o.integrity=e.integrity),e.referrerPolicy&&(o.referrerPolicy=e.referrerPolicy),e.crossOrigin==="use-credentials"?o.credentials="include":e.crossOrigin==="anonymous"?o.credentials="omit":o.credentials="same-origin",o}function i(e){if(e.ep)return;e.ep=!0;const o=a(e);fetch(e.href,o)}})();class d extends Error{constructor(s,a){super(a),this.status=s,this.name="ApiError"}}async function r(t,s,a){const i={method:t,headers:{}};a!==void 0&&(i.body=JSON.stringify(a),i.headers["Content-Type"]="application/json");const e=await fetch(s,i),o=await e.text();let n;if(o.length>0)try{n=JSON.parse(o)}catch{}if(!e.ok){const l=n?.error??o??`HTTP ${e.status}`;throw new d(e.status,l)}return n}const u={listTasks:()=>r("GET","/api/tasks"),getTask:t=>r("GET",`/api/tasks/${t}`),createTask:t=>r("POST","/api/tasks",t),patchTask:(t,s)=>r("PATCH",`/api/tasks/${t}`,s),toggleTask:t=>r("POST",`/api/tasks/${t}/toggle`),deleteTask:t=>r("DELETE",`/api/tasks/${t}`),stats:()=>r("GET","/api/stats"),health:()=>r("GET","/api/health")},c=document.getElementById("root");if(!c)throw new Error("missing #root");c.innerHTML=`
  <div class="app" data-app>
    <header class="topbar">
      <h1>tsk<span class="dot">// loading</span></h1>
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
      <span data-build>tsk web</span>
    </footer>
  </div>
`;async function p(){const t=document.querySelector("[data-file]"),s=document.querySelector(".topbar h1 .dot");try{const a=await u.health();t.textContent=a.file,s.textContent="// ready",s.style.color="var(--color-text-faint)"}catch(a){s.textContent="// offline",s.style.color="var(--color-prio-urgent)",f(a)}}function f(t){const s=document.querySelector("[data-content]"),a=t instanceof Error?t.message:String(t);s.innerHTML=`
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${h(a)}</code>
    </div>`}function h(t){return t.replace(/[&<>"']/g,s=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[s])}p();
