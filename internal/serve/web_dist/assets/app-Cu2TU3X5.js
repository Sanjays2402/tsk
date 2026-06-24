(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const r of document.querySelectorAll('link[rel="modulepreload"]'))s(r);new MutationObserver(r=>{for(const i of r)if(i.type==="childList")for(const a of i.addedNodes)a.tagName==="LINK"&&a.rel==="modulepreload"&&s(a)}).observe(document,{childList:!0,subtree:!0});function n(r){const i={};return r.integrity&&(i.integrity=r.integrity),r.referrerPolicy&&(i.referrerPolicy=r.referrerPolicy),r.crossOrigin==="use-credentials"?i.credentials="include":r.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function s(r){if(r.ep)return;r.ep=!0;const i=n(r);fetch(r.href,i)}})();class O extends Error{constructor(t,n){super(n),this.status=t,this.name="ApiError"}}async function f(e,t,n){const s={method:e,headers:{}};n!==void 0&&(s.body=JSON.stringify(n),s.headers["Content-Type"]="application/json");const r=await fetch(t,s),i=await r.text();let a;if(i.length>0)try{a=JSON.parse(i)}catch{}if(!r.ok){const l=a?.error??i??`HTTP ${r.status}`;throw new O(r.status,l)}return a}const D={listTasks:()=>f("GET","/api/tasks"),getTask:e=>f("GET",`/api/tasks/${e}`),createTask:e=>f("POST","/api/tasks",e),patchTask:(e,t)=>f("PATCH",`/api/tasks/${e}`,t),toggleTask:e=>f("POST",`/api/tasks/${e}/toggle`),deleteTask:e=>f("DELETE",`/api/tasks/${e}`),stats:()=>f("GET","/api/stats"),health:()=>f("GET","/api/health")},j=[{key:"overdue",label:"Overdue"},{key:"today",label:"Today"},{key:"upcoming",label:"Upcoming"},{key:"nodue",label:"No Due"},{key:"done",label:"Done"}],N={urgent:0,high:1,medium:2,low:3};function q(e){if(!e)return null;const[t,n,s]=e.split("-").map(r=>parseInt(r,10));return!t||!n||!s?null:Math.floor(new Date(t,n-1,s).getTime()/864e5)}function F(e,t){if(e.done)return"done";const n=q(e.due);if(n===null)return"nodue";const s=Math.floor(new Date(t.getFullYear(),t.getMonth(),t.getDate()).getTime()/864e5);return n<s?"overdue":n===s?"today":"upcoming"}function b(e,t){const n=N[e.priority]??9,s=N[t.priority]??9;return n!==s?n-s:e.id-t.id}function K(e,t){return e.completed&&t.completed?e.completed!==t.completed?e.completed<t.completed?1:-1:t.id-e.id:e.completed?-1:t.completed?1:t.id-e.id}function G(e,t){const n={overdue:[],today:[],upcoming:[],nodue:[],done:[]};for(const r of e)n[F(r,t)].push(r);n.overdue.sort(b),n.today.sort(b),n.upcoming.sort(b),n.nodue.sort(b),n.done.sort(K);const s=[];for(const{key:r,label:i}of j)n[r].length>0&&s.push({key:r,label:i,tasks:n[r]});return s}function U(e){return e.flatMap(t=>t.tasks)}function g(e){return e.replace(/[&<>"']/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[t])}function Y(e,t){if(!e)return null;const[n,s,r]=e.split("-").map(d=>parseInt(d,10));if(!n||!s||!r)return e;const i=new Date(n,s-1,r),a=new Date(t.getFullYear(),t.getMonth(),t.getDate()),l=Math.round((i.getTime()-a.getTime())/864e5);return l===0?"today":l===1?"tomorrow":l===-1?"yesterday":l<0?`${-l}d ago`:l<7?`in ${l}d`:l<14?"next week":i.toLocaleDateString(void 0,{month:"short",day:"numeric"})}function _(e,t,n){if(!e||t)return"";const[s,r,i]=e.split("-").map(d=>parseInt(d,10));if(!s||!r||!i)return"";const a=new Date(s,r-1,i).getTime(),l=new Date(n.getFullYear(),n.getMonth(),n.getDate()).getTime();return a<l?"is-overdue":a===l?"is-today":""}function B(e,t){const n=_(e.due,e.done,t),s=["row",e.done?"is-done":"",n].filter(Boolean).join(" "),r=e.due?Y(e.due,t):null,i=e.tags.map(a=>`<span class="tag">${g(a)}</span>`).join("");return`
    <li class="${s}" data-id="${e.id}">
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${e.done?"checked":""}>
      <div class="title-wrap">
        <span class="title" title="${g(e.title)}">${g(e.title)}</span>
        <span class="id">#${e.id}</span>
      </div>
      <div class="meta">
        ${i?`<span class="tags">${i}</span>`:""}
        ${r?`<span class="due" title="${g(e.due??"")}">${g(r)}</span>`:""}
        <span class="priority ${g(e.priority)}" title="${g(e.priority)} priority">${J(e.priority)}</span>
      </div>
    </li>`}function J(e){switch(e){case"urgent":return"U";case"high":return"H";case"low":return"L";default:return"M"}}function W(e,t){return e.length===0?`
      <div class="empty">
        <div class="glyph">✓</div>
        <div>No tasks yet.</div>
        <div class="hint">Add one above, or from the CLI: <code>tsk add "buy milk"</code></div>
      </div>`:e.map(n=>{const s=n.tasks.map(r=>B(r,t)).join("");return`
      <section class="section section-${n.key}" data-section="${n.key}">
        <div class="section-head">
          <span class="section-label">${g(n.label)}</span>
          <span class="section-count">${n.tasks.length}</span>
        </div>
        <ul class="list">${s}</ul>
      </section>`}).join("")}function z(e){let t=0,n=0;for(const s of e)s.done?t++:n++;return`<strong>${n}</strong> undone &middot; <strong>${t}</strong> done &middot; <strong>${e.length}</strong> total`}const Q={l:"low",low:"low",m:"medium",med:"medium",medium:"medium",h:"high",high:"high",u:"urgent",urgent:"urgent",critical:"urgent"};function x(e){const t=e.trim().split(/\s+/).filter(Boolean),n=[],s=[];let r,i;for(const a of t){const l=a[0],d=a.slice(1);if(l==="#"&&d.length>0){const h=d.toLowerCase();s.includes(h)||s.push(h);continue}if(l==="!"&&d.length>0){const h=Q[d.toLowerCase()];if(h){r=h;continue}}if(l==="@"&&d.length>0){i=d;continue}n.push(a)}return{title:n.join(" "),priority:r,due:i,tags:s}}function H(e){return e.title.trim().length>0}function I(e){return e.replace(/[&<>"']/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[t])}function V(e){return{urgent:"U",high:"H",low:"L",medium:"M"}[e]??"M"}function X(e){const t=[];e.priority&&t.push(`<span class="pill prio ${e.priority}" title="${e.priority} priority">${V(e.priority)}</span>`),e.due&&t.push(`<span class="pill due">@${I(e.due)}</span>`);for(const s of e.tags)t.push(`<span class="pill tag">#${I(s)}</span>`);return t.length===0?"":(e.title.trim()?`<span class="ghost">${I(e.title.trim())}</span>`:'<span class="ghost">(needs a title)</span>')+t.join("")}function Z(){return{selectedId:null}}function ee(e,t,n=[]){if(t.length===0)return{selectedId:null};if(e.selectedId!==null&&t.includes(e.selectedId))return e;const s=e.selectedId===null?-1:n.indexOf(e.selectedId);if(s>=0){const r=Math.min(s,t.length-1);return{selectedId:t[r]}}return{selectedId:t[0]}}function te(e,t,n){if(t.length===0)return{selectedId:null};if(n==="first")return{selectedId:t[0]};if(n==="last")return{selectedId:t[t.length-1]};const s=e.selectedId===null?-1:t.indexOf(e.selectedId);if(s<0)return{selectedId:n==="next"?t[0]:t[t.length-1]};const r=n==="next"?1:-1,i=Math.max(0,Math.min(t.length-1,s+r));return{selectedId:t[i]}}function ne(e,t,n){return t.includes(n)?{selectedId:n}:e}const P=document.getElementById("root");if(!P)throw new Error("missing #root");P.innerHTML=`
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
      <span data-build><kbd class="kbd-hint" data-help-open>?</kbd> shortcuts &middot; <a href="/api/tasks" style="color:inherit">api</a></span>
    </footer>
  </div>
`;const o={content:u("[data-content]"),file:u("[data-file]"),dot:u("[data-dot]"),count:u("[data-count]"),composer:u("[data-composer]"),field:u("[data-field]"),input:u("[data-input]"),preview:u("[data-preview]")};function u(e){const t=document.querySelector(e);if(!t)throw new Error(`missing ${e}`);return t}let p=[];const E=new Set;let c=Z(),v=[];function T(){const e=new Date,t=G(p,e),n=v;v=U(t).map(s=>s.id),c=ee(c,v,n),o.content.innerHTML=W(t,e),o.count.innerHTML=z(p),M()}function M(){o.content.querySelectorAll("[data-id]").forEach(t=>{const n=Number(t.dataset.id)===c.selectedId;t.classList.toggle("is-selected",n),n?t.setAttribute("aria-current","true"):t.removeAttribute("aria-current")}),c.selectedId!==null&&o.content.querySelector(`[data-id="${c.selectedId}"]`)?.scrollIntoView({block:"nearest"})}async function k(){try{const{file:e,tasks:t}=await D.listTasks();p=t,o.file.textContent=e,m("ready",!1),T()}catch(e){se(e)}}function m(e,t){o.dot.textContent=`// ${e}`,o.dot.style.color=t?"var(--color-prio-urgent)":"var(--color-text-faint)"}function se(e){const t=S(e);m("offline",!0),o.content.innerHTML=`
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${L(t)}</code>
    </div>`}function S(e){return e instanceof O?`${e.status}: ${e.message}`:e instanceof Error?e.message:String(e)}function L(e){return e.replace(/[&<>"']/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[t])}async function A(e){if(E.has(e))return;const t=p.findIndex(s=>s.id===e);if(t<0)return;const n=p[t];p[t]={...n,done:!n.done},E.add(e),T();try{const s=await D.toggleTask(e);p[t]=s,T()}catch(s){p[t]=n,T(),m(`toggle failed: ${S(s)}`,!0),setTimeout(()=>m("ready",!1),3e3)}finally{E.delete(e)}}function $(){const e=x(o.input.value);o.preview.innerHTML=X(e),o.field.classList.toggle("can-submit",H(e))}async function C(){const e=o.input.value,t=x(e);if(H(t)){o.input.value="",$(),m("adding…",!1);try{await D.createTask({title:t.title,priority:t.priority,due:t.due,tags:t.tags.length?t.tags:void 0}),await k()}catch(n){o.input.value=e,$(),m(`add failed: ${S(n)}`,!0),setTimeout(()=>m("ready",!1),4e3),o.input.focus()}}}o.input.addEventListener("input",$);o.composer.addEventListener("submit",e=>{e.preventDefault(),C()});o.input.addEventListener("keydown",e=>{e.key==="Escape"&&(o.input.value="",$(),o.input.blur())});o.content.addEventListener("change",e=>{const t=e.target;if(!t||!(t instanceof HTMLInputElement)||t.dataset.toggle===void 0)return;const n=t.closest("[data-id]");if(!n)return;const s=Number(n.dataset.id);!Number.isFinite(s)||s<=0||A(s)});o.content.addEventListener("click",e=>{const t=e.target;if(!t||t instanceof HTMLInputElement)return;const n=t.closest("[data-id]");if(!n)return;const s=Number(n.dataset.id);!Number.isFinite(s)||s<=0||(c=ne(c,v,s),M())});document.addEventListener("visibilitychange",()=>{document.visibilityState==="visible"&&k()});function re(e){const t=e;return!!t&&(t.tagName==="INPUT"||t.tagName==="TEXTAREA"||t.isContentEditable)}function w(e){c=te(c,v,e),M()}document.addEventListener("keydown",e=>{if(R){(e.key==="Escape"||e.key==="?")&&(e.preventDefault(),y(!1));return}if(!(e.metaKey||e.ctrlKey||e.altKey)&&!re(e.target))switch(e.key){case"j":case"ArrowDown":e.preventDefault(),w("next");break;case"k":case"ArrowUp":e.preventDefault(),w("prev");break;case"g":case"Home":e.preventDefault(),w("first");break;case"G":case"End":e.preventDefault(),w("last");break;case" ":case"Enter":c.selectedId!==null&&(e.preventDefault(),A(c.selectedId));break;case"r":e.preventDefault(),k();break;case"n":e.preventDefault(),o.input.focus();break;case"?":e.preventDefault(),y(!0);break}});let R=!1;const ie=[["j / ↓","Move selection down"],["k / ↑","Move selection up"],["g / G","Jump to first / last"],["space / enter","Toggle the selected task done"],["n","Focus the add-task field"],["r","Refresh from disk"],["esc","Clear the add field / close this help"],["?","Toggle this help"]];function oe(){let e=document.querySelector("[data-help]");return e||(e=document.createElement("div"),e.className="help-overlay",e.setAttribute("data-help",""),e.setAttribute("role","dialog"),e.setAttribute("aria-modal","true"),e.setAttribute("aria-label","Keyboard shortcuts"),e.innerHTML=`
    <div class="help-card">
      <div class="help-title">Keyboard shortcuts</div>
      <dl class="help-list">
        ${ie.map(([t,n])=>`<div class="help-row"><dt><kbd>${L(t)}</kbd></dt><dd>${L(n)}</dd></div>`).join("")}
      </dl>
      <div class="help-foot">Press <kbd>?</kbd> or <kbd>esc</kbd> to close</div>
    </div>`,e.addEventListener("click",t=>{t.target===e&&y(!1)}),document.body.appendChild(e),e)}function y(e){R=e,oe().classList.toggle("is-open",e)}u("[data-help-open]").addEventListener("click",()=>y(!0));window.tsk={refresh:k,tasks:()=>p,toggle:A,add:async e=>{o.input.value=e,await C()},selected:()=>c.selectedId,help:y};k();
