(function(){const t=document.createElement("link").relList;if(t&&t.supports&&t.supports("modulepreload"))return;for(const a of document.querySelectorAll('link[rel="modulepreload"]'))s(a);new MutationObserver(a=>{for(const r of a)if(r.type==="childList")for(const i of r.addedNodes)i.tagName==="LINK"&&i.rel==="modulepreload"&&s(i)}).observe(document,{childList:!0,subtree:!0});function n(a){const r={};return a.integrity&&(r.integrity=a.integrity),a.referrerPolicy&&(r.referrerPolicy=a.referrerPolicy),a.crossOrigin==="use-credentials"?r.credentials="include":a.crossOrigin==="anonymous"?r.credentials="omit":r.credentials="same-origin",r}function s(a){if(a.ep)return;a.ep=!0;const r=n(a);fetch(a.href,r)}})();class j extends Error{constructor(t,n){super(n),this.status=t,this.name="ApiError"}}async function g(e,t,n){const s={method:e,headers:{}};n!==void 0&&(s.body=JSON.stringify(n),s.headers["Content-Type"]="application/json");const a=await fetch(t,s),r=await a.text();let i;if(r.length>0)try{i=JSON.parse(r)}catch{}if(!a.ok){const l=i?.error??r??`HTTP ${a.status}`;throw new j(a.status,l)}return i}const L={listTasks:()=>g("GET","/api/tasks"),getTask:e=>g("GET",`/api/tasks/${e}`),createTask:e=>g("POST","/api/tasks",e),patchTask:(e,t)=>g("PATCH",`/api/tasks/${e}`,t),toggleTask:e=>g("POST",`/api/tasks/${e}/toggle`),deleteTask:e=>g("DELETE",`/api/tasks/${e}`),stats:()=>g("GET","/api/stats"),health:()=>g("GET","/api/health")},J=[{key:"overdue",label:"Overdue"},{key:"today",label:"Today"},{key:"upcoming",label:"Upcoming"},{key:"nodue",label:"No Due"},{key:"done",label:"Done"}],P={urgent:0,high:1,medium:2,low:3};function W(e){if(!e)return null;const[t,n,s]=e.split("-").map(a=>parseInt(a,10));return!t||!n||!s?null:Math.floor(new Date(t,n-1,s).getTime()/864e5)}function z(e,t){if(e.done)return"done";const n=W(e.due);if(n===null)return"nodue";const s=Math.floor(new Date(t.getFullYear(),t.getMonth(),t.getDate()).getTime()/864e5);return n<s?"overdue":n===s?"today":"upcoming"}function $(e,t){const n=P[e.priority]??9,s=P[t.priority]??9;return n!==s?n-s:e.id-t.id}function Q(e,t){return e.completed&&t.completed?e.completed!==t.completed?e.completed<t.completed?1:-1:t.id-e.id:e.completed?-1:t.completed?1:t.id-e.id}function V(e,t){const n={overdue:[],today:[],upcoming:[],nodue:[],done:[]};for(const a of e)n[z(a,t)].push(a);n.overdue.sort($),n.today.sort($),n.upcoming.sort($),n.nodue.sort($),n.done.sort(Q);const s=[];for(const{key:a,label:r}of J)n[a].length>0&&s.push({key:a,label:r,tasks:n[a]});return s}function X(e){return e.flatMap(t=>t.tasks)}function h(e){return e.replace(/[&<>"']/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[t])}function Z(e,t){if(!e)return null;const[n,s,a]=e.split("-").map(p=>parseInt(p,10));if(!n||!s||!a)return e;const r=new Date(n,s-1,a),i=new Date(t.getFullYear(),t.getMonth(),t.getDate()),l=Math.round((r.getTime()-i.getTime())/864e5);return l===0?"today":l===1?"tomorrow":l===-1?"yesterday":l<0?`${-l}d ago`:l<7?`in ${l}d`:l<14?"next week":r.toLocaleDateString(void 0,{month:"short",day:"numeric"})}function ee(e,t,n){if(!e||t)return"";const[s,a,r]=e.split("-").map(p=>parseInt(p,10));if(!s||!a||!r)return"";const i=new Date(s,a-1,r).getTime(),l=new Date(n.getFullYear(),n.getMonth(),n.getDate()).getTime();return i<l?"is-overdue":i===l?"is-today":""}function te(e,t){const n=ee(e.due,e.done,t),s=["row",e.done?"is-done":"",n].filter(Boolean).join(" "),a=e.due?Z(e.due,t):null,r=e.tags.map(i=>`<span class="tag">${h(i)}</span>`).join("");return`
    <li class="${s}" data-id="${e.id}">
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${e.done?"checked":""}>
      <div class="title-wrap">
        <span class="title" title="${h(e.title)}">${h(e.title)}</span>
        <span class="id">#${e.id}</span>
      </div>
      <div class="meta">
        ${r?`<span class="tags">${r}</span>`:""}
        ${a?`<span class="due" title="${h(e.due??"")}">${h(a)}</span>`:""}
        <span class="priority ${h(e.priority)}" title="${h(e.priority)} priority">${ne(e.priority)}</span>
        <button class="row-del" data-del type="button" aria-label="Delete task" title="Delete (x)">&times;</button>
      </div>
    </li>`}function ne(e){switch(e){case"urgent":return"U";case"high":return"H";case"low":return"L";default:return"M"}}function se(e,t){return e.length===0?`
      <div class="empty">
        <div class="glyph">✓</div>
        <div>No tasks yet.</div>
        <div class="hint">Add one above, or from the CLI: <code>tsk add "buy milk"</code></div>
      </div>`:e.map(n=>{const s=n.tasks.map(a=>te(a,t)).join("");return`
      <section class="section section-${n.key}" data-section="${n.key}">
        <div class="section-head">
          <span class="section-label">${h(n.label)}</span>
          <span class="section-count">${n.tasks.length}</span>
        </div>
        <ul class="list">${s}</ul>
      </section>`}).join("")}function ae(e){let t=0,n=0;for(const s of e)s.done?t++:n++;return`<strong>${n}</strong> undone &middot; <strong>${t}</strong> done &middot; <strong>${e.length}</strong> total`}const re={l:"low",low:"low",m:"medium",med:"medium",medium:"medium",h:"high",high:"high",u:"urgent",urgent:"urgent",critical:"urgent"};function F(e){const t=e.trim().split(/\s+/).filter(Boolean),n=[],s=[];let a,r;for(const i of t){const l=i[0],p=i.slice(1);if(l==="#"&&p.length>0){const b=p.toLowerCase();s.includes(b)||s.push(b);continue}if(l==="!"&&p.length>0){const b=re[p.toLowerCase()];if(b){a=b;continue}}if(l==="@"&&p.length>0){r=p;continue}n.push(i)}return{title:n.join(" "),priority:a,due:r,tags:s}}function U(e){return e.title.trim().length>0}function S(e){return e.replace(/[&<>"']/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[t])}function oe(e){return{urgent:"U",high:"H",low:"L",medium:"M"}[e]??"M"}function ie(e){const t=[];e.priority&&t.push(`<span class="pill prio ${e.priority}" title="${e.priority} priority">${oe(e.priority)}</span>`),e.due&&t.push(`<span class="pill due">@${S(e.due)}</span>`);for(const s of e.tags)t.push(`<span class="pill tag">#${S(s)}</span>`);return t.length===0?"":(e.title.trim()?`<span class="ghost">${S(e.title.trim())}</span>`:'<span class="ghost">(needs a title)</span>')+t.join("")}function le(){return{selectedId:null}}function ce(e,t,n=[]){if(t.length===0)return{selectedId:null};if(e.selectedId!==null&&t.includes(e.selectedId))return e;const s=e.selectedId===null?-1:n.indexOf(e.selectedId);if(s>=0){const a=Math.min(s,t.length-1);return{selectedId:t[a]}}return{selectedId:t[0]}}function de(e,t,n){if(t.length===0)return{selectedId:null};if(n==="first")return{selectedId:t[0]};if(n==="last")return{selectedId:t[t.length-1]};const s=e.selectedId===null?-1:t.indexOf(e.selectedId);if(s<0)return{selectedId:n==="next"?t[0]:t[t.length-1]};const a=n==="next"?1:-1,r=Math.max(0,Math.min(t.length-1,s+a));return{selectedId:t[r]}}function K(e,t,n){return t.includes(n)?{selectedId:n}:e}function C(e){return e.replace(/[&<>"']/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[t])}function ue(e){const t=e.actionLabel?`<button class="toast-action" data-toast-action type="button">${C(e.actionLabel)}</button>`:"",n=e.seconds&&e.seconds>0?`<div class="toast-bar" style="animation-duration:${e.seconds}s"></div>`:"";return`
    <div class="toast-body">
      <span class="toast-msg">${C(e.message)}</span>
      ${t}
    </div>
    ${n}`}function pe(e){return`Deleted "${e.length>40?e.slice(0,39)+"…":e}"`}const G=document.getElementById("root");if(!G)throw new Error("missing #root");G.innerHTML=`
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
`;const o={content:f("[data-content]"),file:f("[data-file]"),dot:f("[data-dot]"),count:f("[data-count]"),composer:f("[data-composer]"),field:f("[data-field]"),input:f("[data-input]"),preview:f("[data-preview]")};function f(e){const t=document.querySelector(e);if(!t)throw new Error(`missing ${e}`);return t}let d=[];const A=new Set;let c=le(),y=[];const k=new Set;function v(){const e=new Date,t=d.filter(a=>!k.has(a.id)),n=V(t,e),s=y;y=X(n).map(a=>a.id),c=ce(c,y,s),o.content.innerHTML=se(n,e),o.count.innerHTML=ae(t),D()}function D(){o.content.querySelectorAll("[data-id]").forEach(t=>{const n=Number(t.dataset.id)===c.selectedId;t.classList.toggle("is-selected",n),n?t.setAttribute("aria-current","true"):t.removeAttribute("aria-current")}),c.selectedId!==null&&o.content.querySelector(`[data-id="${c.selectedId}"]`)?.scrollIntoView({block:"nearest"})}async function T(){try{const{file:e,tasks:t}=await L.listTasks();d=t,o.file.textContent=e,m("ready",!1),v()}catch(e){fe(e)}}function m(e,t){o.dot.textContent=`// ${e}`,o.dot.style.color=t?"var(--color-prio-urgent)":"var(--color-text-faint)"}function fe(e){const t=M(e);m("offline",!0),o.content.innerHTML=`
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${x(t)}</code>
    </div>`}function M(e){return e instanceof j?`${e.status}: ${e.message}`:e instanceof Error?e.message:String(e)}function x(e){return e.replace(/[&<>"']/g,t=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"})[t])}async function N(e){if(A.has(e))return;const t=d.findIndex(s=>s.id===e);if(t<0)return;const n=d[t];d[t]={...n,done:!n.done},A.add(e),v();try{const s=await L.toggleTask(e);d[t]=s,v()}catch(s){d[t]=n,v(),m(`toggle failed: ${M(s)}`,!0),setTimeout(()=>m("ready",!1),3e3)}finally{A.delete(e)}}let u=null;const q=5;function me(){let e=document.querySelector("[data-toast]");return e||(e=document.createElement("div"),e.className="toast",e.setAttribute("data-toast",""),e.setAttribute("role","status"),e.setAttribute("aria-live","polite"),e.addEventListener("click",t=>{t.target?.dataset.toastAction!==void 0&&H()}),document.body.appendChild(e),e)}function Y(){document.querySelector("[data-toast]")?.classList.remove("is-open")}function O(e){const t=d.find(a=>a.id===e);if(!t)return;u&&R(),k.add(e),v();const n=me();n.innerHTML=ue({message:pe(t.title),actionLabel:"Undo",seconds:q}),n.classList.add("is-open");const s=window.setTimeout(R,q*1e3);u={task:t,timer:s}}async function R(){if(!u)return;const{task:e,timer:t}=u;window.clearTimeout(t),u=null,Y();try{await L.deleteTask(e.id),d=d.filter(n=>n.id!==e.id),k.delete(e.id),v()}catch(n){k.delete(e.id),v(),m(`delete failed: ${M(n)}`,!0),setTimeout(()=>m("ready",!1),4e3)}}function H(){if(!u)return;window.clearTimeout(u.timer),k.delete(u.task.id);const e=u.task.id;u=null,Y(),v(),c=K(c,y,e),D()}function I(){const e=F(o.input.value);o.preview.innerHTML=ie(e),o.field.classList.toggle("can-submit",U(e))}async function _(){const e=o.input.value,t=F(e);if(U(t)){o.input.value="",I(),m("adding…",!1);try{await L.createTask({title:t.title,priority:t.priority,due:t.due,tags:t.tags.length?t.tags:void 0}),await T()}catch(n){o.input.value=e,I(),m(`add failed: ${M(n)}`,!0),setTimeout(()=>m("ready",!1),4e3),o.input.focus()}}}o.input.addEventListener("input",I);o.composer.addEventListener("submit",e=>{e.preventDefault(),_()});o.input.addEventListener("keydown",e=>{e.key==="Escape"&&(o.input.value="",I(),o.input.blur())});o.content.addEventListener("change",e=>{const t=e.target;if(!t||!(t instanceof HTMLInputElement)||t.dataset.toggle===void 0)return;const n=t.closest("[data-id]");if(!n)return;const s=Number(n.dataset.id);!Number.isFinite(s)||s<=0||N(s)});o.content.addEventListener("click",e=>{const t=e.target;if(!t||t instanceof HTMLInputElement)return;const n=t.closest("[data-id]");if(!n)return;const s=Number(n.dataset.id);if(!(!Number.isFinite(s)||s<=0)){if(t.closest("[data-del]")){O(s);return}c=K(c,y,s),D()}});document.addEventListener("visibilitychange",()=>{document.visibilityState==="visible"&&T()});function ge(e){const t=e;return!!t&&(t.tagName==="INPUT"||t.tagName==="TEXTAREA"||t.isContentEditable)}function E(e){c=de(c,y,e),D()}document.addEventListener("keydown",e=>{if(B){(e.key==="Escape"||e.key==="?")&&(e.preventDefault(),w(!1));return}if(!(e.metaKey||e.ctrlKey||e.altKey)&&!ge(e.target))switch(e.key){case"j":case"ArrowDown":e.preventDefault(),E("next");break;case"k":case"ArrowUp":e.preventDefault(),E("prev");break;case"g":case"Home":e.preventDefault(),E("first");break;case"G":case"End":e.preventDefault(),E("last");break;case" ":case"Enter":c.selectedId!==null&&(e.preventDefault(),N(c.selectedId));break;case"x":case"Delete":case"Backspace":c.selectedId!==null&&(e.preventDefault(),O(c.selectedId));break;case"u":u&&(e.preventDefault(),H());break;case"r":e.preventDefault(),T();break;case"n":e.preventDefault(),o.input.focus();break;case"?":e.preventDefault(),w(!0);break}});let B=!1;const he=[["j / ↓","Move selection down"],["k / ↑","Move selection up"],["g / G","Jump to first / last"],["space / enter","Toggle the selected task done"],["x / del","Delete the selected task (undoable)"],["u","Undo the last delete"],["n","Focus the add-task field"],["r","Refresh from disk"],["esc","Clear the add field / close this help"],["?","Toggle this help"]];function ve(){let e=document.querySelector("[data-help]");return e||(e=document.createElement("div"),e.className="help-overlay",e.setAttribute("data-help",""),e.setAttribute("role","dialog"),e.setAttribute("aria-modal","true"),e.setAttribute("aria-label","Keyboard shortcuts"),e.innerHTML=`
    <div class="help-card">
      <div class="help-title">Keyboard shortcuts</div>
      <dl class="help-list">
        ${he.map(([t,n])=>`<div class="help-row"><dt><kbd>${x(t)}</kbd></dt><dd>${x(n)}</dd></div>`).join("")}
      </dl>
      <div class="help-foot">Press <kbd>?</kbd> or <kbd>esc</kbd> to close</div>
    </div>`,e.addEventListener("click",t=>{t.target===e&&w(!1)}),document.body.appendChild(e),e)}function w(e){B=e,ve().classList.toggle("is-open",e)}f("[data-help-open]").addEventListener("click",()=>w(!0));window.tsk={refresh:T,tasks:()=>d,toggle:N,add:async e=>{o.input.value=e,await _()},selected:()=>c.selectedId,help:w,del:O,undo:H};T();
