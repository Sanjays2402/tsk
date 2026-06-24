(function(){const e=document.createElement("link").relList;if(e&&e.supports&&e.supports("modulepreload"))return;for(const n of document.querySelectorAll('link[rel="modulepreload"]'))i(n);new MutationObserver(n=>{for(const r of n)if(r.type==="childList")for(const a of r.addedNodes)a.tagName==="LINK"&&a.rel==="modulepreload"&&i(a)}).observe(document,{childList:!0,subtree:!0});function s(n){const r={};return n.integrity&&(r.integrity=n.integrity),n.referrerPolicy&&(r.referrerPolicy=n.referrerPolicy),n.crossOrigin==="use-credentials"?r.credentials="include":n.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function i(n){if(n.ep)return;n.ep=!0;const r=s(n);fetch(n.href,r)}})();class b extends Error{constructor(e,s){super(s),this.status=e,this.name="ApiError"}}async function u(t,e,s){const i={method:t,headers:{}};s!==void 0&&(i.body=JSON.stringify(s),i.headers["Content-Type"]="application/json");const n=await fetch(e,i),r=await n.text();let a;if(r.length>0)try{a=JSON.parse(r)}catch{}if(!n.ok){const c=a?.error??r??`HTTP ${n.status}`;throw new b(n.status,c)}return a}const k={listTasks:()=>u("GET","/api/tasks"),getTask:t=>u("GET",`/api/tasks/${t}`),createTask:t=>u("POST","/api/tasks",t),patchTask:(t,e)=>u("PATCH",`/api/tasks/${t}`,e),toggleTask:t=>u("POST",`/api/tasks/${t}/toggle`),deleteTask:t=>u("DELETE",`/api/tasks/${t}`),stats:()=>u("GET","/api/stats"),health:()=>u("GET","/api/health")};function f(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}function D(t,e){if(!t)return null;const[s,i,n]=t.split("-").map(l=>parseInt(l,10));if(!s||!i||!n)return t;const r=new Date(s,i-1,n),a=new Date(e.getFullYear(),e.getMonth(),e.getDate()),c=Math.round((r.getTime()-a.getTime())/864e5);return c===0?"today":c===1?"tomorrow":c===-1?"yesterday":c<0?`${-c}d ago`:c<7?`in ${c}d`:c<14?"next week":r.toLocaleDateString(void 0,{month:"short",day:"numeric"})}function C(t,e,s){if(!t||e)return"";const[i,n,r]=t.split("-").map(l=>parseInt(l,10));if(!i||!n||!r)return"";const a=new Date(i,n-1,r).getTime(),c=new Date(s.getFullYear(),s.getMonth(),s.getDate()).getTime();return a<c?"is-overdue":a===c?"is-today":""}function H(t,e){const s=C(t.due,t.done,e),i=["row",t.done?"is-done":"",s].filter(Boolean).join(" "),n=t.due?D(t.due,e):null,r=t.tags.map(a=>`<span class="tag">${f(a)}</span>`).join("");return`
    <li class="${i}" data-id="${t.id}">
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${t.done?"checked":""}>
      <div class="title-wrap">
        <span class="title" title="${f(t.title)}">${f(t.title)}</span>
        <span class="id">#${t.id}</span>
      </div>
      <div class="meta">
        ${r?`<span class="tags">${r}</span>`:""}
        ${n?`<span class="due" title="${f(t.due??"")}">${f(n)}</span>`:""}
        <span class="priority ${f(t.priority)}" title="${f(t.priority)} priority">${x(t.priority)}</span>
      </div>
    </li>`}function x(t){switch(t){case"urgent":return"U";case"high":return"H";case"low":return"L";default:return"M"}}function A(t,e){return t.length===0?`
      <div class="empty">
        <div class="glyph">✓</div>
        <div>No tasks yet.</div>
        <div class="hint">Add one from the CLI: <code>tsk add "buy milk"</code></div>
      </div>`:`<ul class="list">${[...t].sort((n,r)=>{if(n.done!==r.done)return n.done?1:-1;const a={urgent:0,high:1,medium:2,low:3},c=a[n.priority]??9,l=a[r.priority]??9;return c!==l?c-l:n.id-r.id}).map(n=>H(n,e)).join("")}</ul>`}function O(t){let e=0,s=0;for(const i of t)i.done?e++:s++;return`<strong>${s}</strong> undone &middot; <strong>${e}</strong> done &middot; <strong>${t.length}</strong> total`}const N={l:"low",low:"low",m:"medium",med:"medium",medium:"medium",h:"high",high:"high",u:"urgent",urgent:"urgent",critical:"urgent"};function L(t){const e=t.trim().split(/\s+/).filter(Boolean),s=[],i=[];let n,r;for(const a of e){const c=a[0],l=a.slice(1);if(c==="#"&&l.length>0){const m=l.toLowerCase();i.includes(m)||i.push(m);continue}if(c==="!"&&l.length>0){const m=N[l.toLowerCase()];if(m){n=m;continue}}if(c==="@"&&l.length>0){r=l;continue}s.push(a)}return{title:s.join(" "),priority:n,due:r,tags:i}}function E(t){return t.title.trim().length>0}function w(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}function I(t){return{urgent:"U",high:"H",low:"L",medium:"M"}[t]??"M"}function q(t){const e=[];t.priority&&e.push(`<span class="pill prio ${t.priority}" title="${t.priority} priority">${I(t.priority)}</span>`),t.due&&e.push(`<span class="pill due">@${w(t.due)}</span>`);for(const i of t.tags)e.push(`<span class="pill tag">#${w(i)}</span>`);return e.length===0?"":(t.title.trim()?`<span class="ghost">${w(t.title.trim())}</span>`:'<span class="ghost">(needs a title)</span>')+e.join("")}const M=document.getElementById("root");if(!M)throw new Error("missing #root");M.innerHTML=`
  <div class="app" data-app>
    <header class="topbar">
      <h1>tsk<span class="dot" data-dot>// loading</span></h1>
      <div class="file" data-file>—</div>
    </header>
    <form class="composer" data-composer autocomplete="off">
      <div class="composer-field" data-field>
        <span class="plus" aria-hidden="true">+</span>
        <input
          class="composer-input"
          data-input
          type="text"
          name="quickadd"
          placeholder="Add a task…  try: ship release !high @fri #work"
          aria-label="Add a task. Use !priority @due #tag for inline metadata."
          spellcheck="false"
        >
        <button class="composer-submit" type="submit" data-submit tabindex="-1">Add</button>
      </div>
      <div class="composer-preview" data-preview></div>
    </form>
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
`;const o={content:p("[data-content]"),file:p("[data-file]"),dot:p("[data-dot]"),count:p("[data-count]"),composer:p("[data-composer]"),field:p("[data-field]"),input:p("[data-input]"),preview:p("[data-preview]")};function p(t){const e=document.querySelector(t);if(!e)throw new Error(`missing ${t}`);return e}let d=[];const T=new Set;function y(){const t=new Date;o.content.innerHTML=A(d,t),o.count.innerHTML=O(d)}async function h(){try{const{file:t,tasks:e}=await k.listTasks();d=e,o.file.textContent=t,g("ready",!1),y()}catch(t){j(t)}}function g(t,e){o.dot.textContent=`// ${t}`,o.dot.style.color=e?"var(--color-prio-urgent)":"var(--color-text-faint)"}function j(t){const e=$(t);g("offline",!0),o.content.innerHTML=`
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${F(e)}</code>
    </div>`}function $(t){return t instanceof b?`${t.status}: ${t.message}`:t instanceof Error?t.message:String(t)}function F(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}async function S(t){if(T.has(t))return;const e=d.findIndex(i=>i.id===t);if(e<0)return;const s=d[e];d[e]={...s,done:!s.done},T.add(t),y();try{const i=await k.toggleTask(t);d[e]=i,y()}catch(i){d[e]=s,y(),g(`toggle failed: ${$(i)}`,!0),setTimeout(()=>g("ready",!1),3e3)}finally{T.delete(t)}}function v(){const t=L(o.input.value);o.preview.innerHTML=q(t),o.field.classList.toggle("can-submit",E(t))}async function P(){const t=o.input.value,e=L(t);if(E(e)){o.input.value="",v(),g("adding…",!1);try{await k.createTask({title:e.title,priority:e.priority,due:e.due,tags:e.tags.length?e.tags:void 0}),await h()}catch(s){o.input.value=t,v(),g(`add failed: ${$(s)}`,!0),setTimeout(()=>g("ready",!1),4e3),o.input.focus()}}}o.input.addEventListener("input",v);o.composer.addEventListener("submit",t=>{t.preventDefault(),P()});o.input.addEventListener("keydown",t=>{t.key==="Escape"&&(o.input.value="",v(),o.input.blur())});o.content.addEventListener("change",t=>{const e=t.target;if(!e||!(e instanceof HTMLInputElement)||e.dataset.toggle===void 0)return;const s=e.closest("[data-id]");if(!s)return;const i=Number(s.dataset.id);!Number.isFinite(i)||i<=0||S(i)});document.addEventListener("visibilitychange",()=>{document.visibilityState==="visible"&&h()});function R(t){const e=t;return!!e&&(e.tagName==="INPUT"||e.tagName==="TEXTAREA"||e.isContentEditable)}document.addEventListener("keydown",t=>{t.metaKey||t.ctrlKey||t.altKey||R(t.target)||(t.key==="r"?(t.preventDefault(),h()):t.key==="n"&&(t.preventDefault(),o.input.focus()))});window.tsk={refresh:h,tasks:()=>d,toggle:S,add:async t=>{o.input.value=t,await P()}};h();
