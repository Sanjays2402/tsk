(function(){const e=document.createElement("link").relList;if(e&&e.supports&&e.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))o(s);new MutationObserver(s=>{for(const n of s)if(n.type==="childList")for(const a of n.addedNodes)a.tagName==="LINK"&&a.rel==="modulepreload"&&o(a)}).observe(document,{childList:!0,subtree:!0});function r(s){const n={};return s.integrity&&(n.integrity=s.integrity),s.referrerPolicy&&(n.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?n.credentials="include":s.crossOrigin==="anonymous"?n.credentials="omit":n.credentials="same-origin",n}function o(s){if(s.ep)return;s.ep=!0;const n=r(s);fetch(s.href,n)}})();class g extends Error{constructor(e,r){super(r),this.status=e,this.name="ApiError"}}async function c(t,e,r){const o={method:t,headers:{}};r!==void 0&&(o.body=JSON.stringify(r),o.headers["Content-Type"]="application/json");const s=await fetch(e,o),n=await s.text();let a;if(n.length>0)try{a=JSON.parse(n)}catch{}if(!s.ok){const i=a?.error??n??`HTTP ${s.status}`;throw new g(s.status,i)}return a}const h={listTasks:()=>c("GET","/api/tasks"),getTask:t=>c("GET",`/api/tasks/${t}`),createTask:t=>c("POST","/api/tasks",t),patchTask:(t,e)=>c("PATCH",`/api/tasks/${t}`,e),toggleTask:t=>c("POST",`/api/tasks/${t}/toggle`),deleteTask:t=>c("DELETE",`/api/tasks/${t}`),stats:()=>c("GET","/api/stats"),health:()=>c("GET","/api/health")};function l(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}function v(t,e){if(!t)return null;const[r,o,s]=t.split("-").map(u=>parseInt(u,10));if(!r||!o||!s)return t;const n=new Date(r,o-1,s),a=new Date(e.getFullYear(),e.getMonth(),e.getDate()),i=Math.round((n.getTime()-a.getTime())/864e5);return i===0?"today":i===1?"tomorrow":i===-1?"yesterday":i<0?`${-i}d ago`:i<7?`in ${i}d`:i<14?"next week":n.toLocaleDateString(void 0,{month:"short",day:"numeric"})}function T(t,e,r){if(!t||e)return"";const[o,s,n]=t.split("-").map(u=>parseInt(u,10));if(!o||!s||!n)return"";const a=new Date(o,s-1,n).getTime(),i=new Date(r.getFullYear(),r.getMonth(),r.getDate()).getTime();return a<i?"is-overdue":a===i?"is-today":""}function $(t,e){const r=T(t.due,t.done,e),o=["row",t.done?"is-done":"",r].filter(Boolean).join(" "),s=t.due?v(t.due,e):null,n=t.tags.map(a=>`<span class="tag">${l(a)}</span>`).join("");return`
    <li class="${o}" data-id="${t.id}">
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${t.done?"checked":""}>
      <div class="title-wrap">
        <span class="title" title="${l(t.title)}">${l(t.title)}</span>
        <span class="id">#${t.id}</span>
      </div>
      <div class="meta">
        ${n?`<span class="tags">${n}</span>`:""}
        ${s?`<span class="due" title="${l(t.due??"")}">${l(s)}</span>`:""}
        <span class="priority ${l(t.priority)}" title="${l(t.priority)} priority">${w(t.priority)}</span>
      </div>
    </li>`}function w(t){switch(t){case"urgent":return"U";case"high":return"H";case"low":return"L";default:return"M"}}function k(t,e){return t.length===0?`
      <div class="empty">
        <div class="glyph">✓</div>
        <div>No tasks yet.</div>
        <div class="hint">Add one from the CLI: <code>tsk add "buy milk"</code></div>
      </div>`:`<ul class="list">${[...t].sort((s,n)=>{if(s.done!==n.done)return s.done?1:-1;const a={urgent:0,high:1,medium:2,low:3},i=a[s.priority]??9,u=a[n.priority]??9;return i!==u?i-u:s.id-n.id}).map(s=>$(s,e)).join("")}</ul>`}function E(t){let e=0,r=0;for(const o of t)o.done?e++:r++;return`<strong>${r}</strong> undone &middot; <strong>${e}</strong> done &middot; <strong>${t.length}</strong> total`}const m=document.getElementById("root");if(!m)throw new Error("missing #root");m.innerHTML=`
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
`;const d={content:p("[data-content]"),file:p("[data-file]"),dot:p("[data-dot]"),count:p("[data-count]")};function p(t){const e=document.querySelector(t);if(!e)throw new Error(`missing ${t}`);return e}let y=[];async function f(){try{const{file:t,tasks:e}=await h.listTasks();y=e,d.file.textContent=t,d.dot.textContent="// ready",d.dot.style.color="var(--color-text-faint)";const r=new Date;d.content.innerHTML=k(e,r),d.count.innerHTML=E(e)}catch(t){b(t)}}function b(t){const e=t instanceof g?`${t.status}: ${t.message}`:t instanceof Error?t.message:String(t);d.dot.textContent="// offline",d.dot.style.color="var(--color-prio-urgent)",d.content.innerHTML=`
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${L(e)}</code>
    </div>`}function L(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}document.addEventListener("visibilitychange",()=>{document.visibilityState==="visible"&&f()});document.addEventListener("keydown",t=>{if(t.key==="r"&&!t.metaKey&&!t.ctrlKey&&!t.altKey){const e=t.target;if(e&&(e.tagName==="INPUT"||e.tagName==="TEXTAREA"||e.isContentEditable))return;t.preventDefault(),f()}});window.tsk={refresh:f,tasks:()=>y};f();
