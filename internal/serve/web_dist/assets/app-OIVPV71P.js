(function(){const e=document.createElement("link").relList;if(e&&e.supports&&e.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))i(s);new MutationObserver(s=>{for(const r of s)if(r.type==="childList")for(const a of r.addedNodes)a.tagName==="LINK"&&a.rel==="modulepreload"&&i(a)}).observe(document,{childList:!0,subtree:!0});function n(s){const r={};return s.integrity&&(r.integrity=s.integrity),s.referrerPolicy&&(r.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?r.credentials="include":s.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function i(s){if(s.ep)return;s.ep=!0;const r=n(s);fetch(s.href,r)}})();class L extends Error{constructor(e,n){super(n),this.status=e,this.name="ApiError"}}async function d(t,e,n){const i={method:t,headers:{}};n!==void 0&&(i.body=JSON.stringify(n),i.headers["Content-Type"]="application/json");const s=await fetch(e,i),r=await s.text();let a;if(r.length>0)try{a=JSON.parse(r)}catch{}if(!s.ok){const c=a?.error??r??`HTTP ${s.status}`;throw new L(s.status,c)}return a}const $={listTasks:()=>d("GET","/api/tasks"),getTask:t=>d("GET",`/api/tasks/${t}`),createTask:t=>d("POST","/api/tasks",t),patchTask:(t,e)=>d("PATCH",`/api/tasks/${t}`,e),toggleTask:t=>d("POST",`/api/tasks/${t}/toggle`),deleteTask:t=>d("DELETE",`/api/tasks/${t}`),stats:()=>d("GET","/api/stats"),health:()=>d("GET","/api/health")},I=[{key:"overdue",label:"Overdue"},{key:"today",label:"Today"},{key:"upcoming",label:"Upcoming"},{key:"nodue",label:"No Due"},{key:"done",label:"Done"}],E={urgent:0,high:1,medium:2,low:3};function N(t){if(!t)return null;const[e,n,i]=t.split("-").map(s=>parseInt(s,10));return!e||!n||!i?null:Math.floor(new Date(e,n-1,i).getTime()/864e5)}function C(t,e){if(t.done)return"done";const n=N(t.due);if(n===null)return"nodue";const i=Math.floor(new Date(e.getFullYear(),e.getMonth(),e.getDate()).getTime()/864e5);return n<i?"overdue":n===i?"today":"upcoming"}function y(t,e){const n=E[t.priority]??9,i=E[e.priority]??9;return n!==i?n-i:t.id-e.id}function A(t,e){return t.completed&&e.completed?t.completed!==e.completed?t.completed<e.completed?1:-1:e.id-t.id:t.completed?-1:e.completed?1:e.id-t.id}function H(t,e){const n={overdue:[],today:[],upcoming:[],nodue:[],done:[]};for(const s of t)n[C(s,e)].push(s);n.overdue.sort(y),n.today.sort(y),n.upcoming.sort(y),n.nodue.sort(y),n.done.sort(A);const i=[];for(const{key:s,label:r}of I)n[s].length>0&&i.push({key:s,label:r,tasks:n[s]});return i}function f(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}function x(t,e){if(!t)return null;const[n,i,s]=t.split("-").map(l=>parseInt(l,10));if(!n||!i||!s)return t;const r=new Date(n,i-1,s),a=new Date(e.getFullYear(),e.getMonth(),e.getDate()),c=Math.round((r.getTime()-a.getTime())/864e5);return c===0?"today":c===1?"tomorrow":c===-1?"yesterday":c<0?`${-c}d ago`:c<7?`in ${c}d`:c<14?"next week":r.toLocaleDateString(void 0,{month:"short",day:"numeric"})}function R(t,e,n){if(!t||e)return"";const[i,s,r]=t.split("-").map(l=>parseInt(l,10));if(!i||!s||!r)return"";const a=new Date(i,s-1,r).getTime(),c=new Date(n.getFullYear(),n.getMonth(),n.getDate()).getTime();return a<c?"is-overdue":a===c?"is-today":""}function F(t,e){const n=R(t.due,t.done,e),i=["row",t.done?"is-done":"",n].filter(Boolean).join(" "),s=t.due?x(t.due,e):null,r=t.tags.map(a=>`<span class="tag">${f(a)}</span>`).join("");return`
    <li class="${i}" data-id="${t.id}">
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${t.done?"checked":""}>
      <div class="title-wrap">
        <span class="title" title="${f(t.title)}">${f(t.title)}</span>
        <span class="id">#${t.id}</span>
      </div>
      <div class="meta">
        ${r?`<span class="tags">${r}</span>`:""}
        ${s?`<span class="due" title="${f(t.due??"")}">${f(s)}</span>`:""}
        <span class="priority ${f(t.priority)}" title="${f(t.priority)} priority">${j(t.priority)}</span>
      </div>
    </li>`}function j(t){switch(t){case"urgent":return"U";case"high":return"H";case"low":return"L";default:return"M"}}function q(t,e){return t.length===0?`
      <div class="empty">
        <div class="glyph">✓</div>
        <div>No tasks yet.</div>
        <div class="hint">Add one above, or from the CLI: <code>tsk add "buy milk"</code></div>
      </div>`:H(t,e).map(i=>{const s=i.tasks.map(r=>F(r,e)).join("");return`
      <section class="section section-${i.key}" data-section="${i.key}">
        <div class="section-head">
          <span class="section-label">${f(i.label)}</span>
          <span class="section-count">${i.tasks.length}</span>
        </div>
        <ul class="list">${s}</ul>
      </section>`}).join("")}function K(t){let e=0,n=0;for(const i of t)i.done?e++:n++;return`<strong>${n}</strong> undone &middot; <strong>${e}</strong> done &middot; <strong>${t.length}</strong> total`}const U={l:"low",low:"low",m:"medium",med:"medium",medium:"medium",h:"high",high:"high",u:"urgent",urgent:"urgent",critical:"urgent"};function D(t){const e=t.trim().split(/\s+/).filter(Boolean),n=[],i=[];let s,r;for(const a of e){const c=a[0],l=a.slice(1);if(c==="#"&&l.length>0){const m=l.toLowerCase();i.includes(m)||i.push(m);continue}if(c==="!"&&l.length>0){const m=U[l.toLowerCase()];if(m){s=m;continue}}if(c==="@"&&l.length>0){r=l;continue}n.push(a)}return{title:n.join(" "),priority:s,due:r,tags:i}}function M(t){return t.title.trim().length>0}function T(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}function Y(t){return{urgent:"U",high:"H",low:"L",medium:"M"}[t]??"M"}function G(t){const e=[];t.priority&&e.push(`<span class="pill prio ${t.priority}" title="${t.priority} priority">${Y(t.priority)}</span>`),t.due&&e.push(`<span class="pill due">@${T(t.due)}</span>`);for(const i of t.tags)e.push(`<span class="pill tag">#${T(i)}</span>`);return e.length===0?"":(t.title.trim()?`<span class="ghost">${T(t.title.trim())}</span>`:'<span class="ghost">(needs a title)</span>')+e.join("")}const S=document.getElementById("root");if(!S)throw new Error("missing #root");S.innerHTML=`
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
`;const o={content:p("[data-content]"),file:p("[data-file]"),dot:p("[data-dot]"),count:p("[data-count]"),composer:p("[data-composer]"),field:p("[data-field]"),input:p("[data-input]"),preview:p("[data-preview]")};function p(t){const e=document.querySelector(t);if(!e)throw new Error(`missing ${t}`);return e}let u=[];const w=new Set;function v(){const t=new Date;o.content.innerHTML=q(u,t),o.count.innerHTML=K(u)}async function h(){try{const{file:t,tasks:e}=await $.listTasks();u=e,o.file.textContent=t,g("ready",!1),v()}catch(t){B(t)}}function g(t,e){o.dot.textContent=`// ${t}`,o.dot.style.color=e?"var(--color-prio-urgent)":"var(--color-text-faint)"}function B(t){const e=b(t);g("offline",!0),o.content.innerHTML=`
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${_(e)}</code>
    </div>`}function b(t){return t instanceof L?`${t.status}: ${t.message}`:t instanceof Error?t.message:String(t)}function _(t){return t.replace(/[&<>"']/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[e])}async function P(t){if(w.has(t))return;const e=u.findIndex(i=>i.id===t);if(e<0)return;const n=u[e];u[e]={...n,done:!n.done},w.add(t),v();try{const i=await $.toggleTask(t);u[e]=i,v()}catch(i){u[e]=n,v(),g(`toggle failed: ${b(i)}`,!0),setTimeout(()=>g("ready",!1),3e3)}finally{w.delete(t)}}function k(){const t=D(o.input.value);o.preview.innerHTML=G(t),o.field.classList.toggle("can-submit",M(t))}async function O(){const t=o.input.value,e=D(t);if(M(e)){o.input.value="",k(),g("adding…",!1);try{await $.createTask({title:e.title,priority:e.priority,due:e.due,tags:e.tags.length?e.tags:void 0}),await h()}catch(n){o.input.value=t,k(),g(`add failed: ${b(n)}`,!0),setTimeout(()=>g("ready",!1),4e3),o.input.focus()}}}o.input.addEventListener("input",k);o.composer.addEventListener("submit",t=>{t.preventDefault(),O()});o.input.addEventListener("keydown",t=>{t.key==="Escape"&&(o.input.value="",k(),o.input.blur())});o.content.addEventListener("change",t=>{const e=t.target;if(!e||!(e instanceof HTMLInputElement)||e.dataset.toggle===void 0)return;const n=e.closest("[data-id]");if(!n)return;const i=Number(n.dataset.id);!Number.isFinite(i)||i<=0||P(i)});document.addEventListener("visibilitychange",()=>{document.visibilityState==="visible"&&h()});function J(t){const e=t;return!!e&&(e.tagName==="INPUT"||e.tagName==="TEXTAREA"||e.isContentEditable)}document.addEventListener("keydown",t=>{t.metaKey||t.ctrlKey||t.altKey||J(t.target)||(t.key==="r"?(t.preventDefault(),h()):t.key==="n"&&(t.preventDefault(),o.input.focus()))});window.tsk={refresh:h,tasks:()=>u,toggle:P,add:async t=>{o.input.value=t,await O()}};h();
