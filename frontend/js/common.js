// Shared helpers for every page. API_BASE comes from config.js, which every page loads first.

// ---------- Login tokens ----------
// The website and the API live on different sites (e.g. vercel.app and onrender.com), where browsers
// do not send login cookies. So each login stores a token and every request sends it as a Bearer header.
const TOKEN_KEYS = { student: 'gc-token', admin: 'gc-admin-token' };
const roleFor = path => (path.startsWith('/api/admin') ? 'admin' : 'student');

function getToken(role) {
  try { return localStorage.getItem(TOKEN_KEYS[role]) || ''; } catch { return ''; }
}

function setToken(role, token) {
  try {
    if (token) localStorage.setItem(TOKEN_KEYS[role], token);
    else localStorage.removeItem(TOKEN_KEYS[role]);
  } catch { /* storage blocked: the cookie (same-site hosting) still works */ }
}

function authHeaders(path) {
  const token = getToken(roleFor(path));
  return token ? { Authorization: 'Bearer ' + token } : {};
}

async function api(path, { method = 'GET', body } = {}) {
  const opts = { method, headers: authHeaders(path), credentials: 'include' };
  if (method !== 'GET') {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body || {});
  }
  let res;
  try {
    res = await fetch(API_BASE + path, opts);
  } catch {
    const err = new Error('Cannot reach the server. Check your internet connection and try again.');
    err.status = 0;
    throw err;
  }
  let data = null;
  try { data = await res.json(); } catch { /* empty body */ }
  if (!res.ok) {
    if (res.status === 401) setToken(roleFor(path), ''); // expired or revoked
    const err = new Error(errorText(data) || 'Something went wrong. Please try again.');
    err.status = res.status;
    throw err;
  }
  if (data && data.token && /\/(login|register)$/.test(path)) setToken(roleFor(path), data.token);
  return data;
}

// Images that need a login (e.g. the generated payment QR) are fetched with the token.
// Mark them with data-auth-src="/api/..." instead of src.
const authImageCache = new Map();
async function loadAuthImage(img) {
  const path = img.dataset.authSrc;
  img.removeAttribute('data-auth-src');
  try {
    if (!authImageCache.has(path)) {
      const res = await fetch(API_BASE + path, { headers: authHeaders(path), credentials: 'include' });
      if (!res.ok) throw new Error(res.status);
      authImageCache.set(path, URL.createObjectURL(await res.blob()));
    }
    img.src = authImageCache.get(path);
  } catch { img.alt = 'Could not load image'; }
}
new MutationObserver(() => document.querySelectorAll('img[data-auth-src]').forEach(loadAuthImage))
  .observe(document.documentElement, { childList: true, subtree: true });

function errorText(data) {
  if (!data || !data.detail) return '';
  if (typeof data.detail === 'string') return data.detail;
  // FastAPI validation errors: [{loc, msg}, ...]
  return data.detail.map(d => {
    const field = (d.loc || []).slice(-1)[0];
    return (field ? field.replace(/_/g, ' ') + ': ' : '') + d.msg.replace(/^Value error, /, '');
  }).join(' · ');
}

// ---------- Formatting ----------

function esc(value) {
  return String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

// Escapes text, then turns https:// links into clickable links.
function linkify(text) {
  return esc(text).replace(/https:\/\/[^\s<]+[^\s<.,;:!?)\]'"]/g,
    url => `<a href="${url}" target="_blank" rel="noopener">${url}</a>`);
}

function fmtDate(ts) {
  if (!ts) return '—';
  return new Date(ts * 1000).toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' });
}

function fmtDateTime(ts) {
  if (!ts) return '—';
  return new Date(ts * 1000).toLocaleString('en-IN', { day: 'numeric', month: 'short', year: 'numeric', hour: 'numeric', minute: '2-digit' });
}

function timeAgo(ts) {
  const s = Math.floor(Date.now() / 1000) - ts;
  if (s < 60) return 'just now';
  if (s < 3600) return Math.floor(s / 60) + ' min ago';
  if (s < 86400) return Math.floor(s / 3600) + ' hr ago';
  if (s < 7 * 86400) return Math.floor(s / 86400) + ' days ago';
  return fmtDate(ts);
}

function rupees(n) {
  return '₹' + Number(n || 0).toLocaleString('en-IN', { maximumFractionDigits: 2 });
}

function initials(name) {
  return String(name || '?').trim().split(/\s+/).slice(0, 2).map(w => w[0]).join('').toUpperCase();
}

// Phone helpers: tel: link and WhatsApp link (assumes India when no country code).
function telHref(phone) { return 'tel:' + String(phone).replace(/[^\d+]/g, ''); }
function waHref(phone, text = '') {
  let digits = String(phone).replace(/\D/g, '');
  if (digits.length === 10) digits = '91' + digits;
  return `https://wa.me/${digits}${text ? '?text=' + encodeURIComponent(text) : ''}`;
}

// ---------- Icons (inline SVG, stroke style) ----------

const ICONS = {
  bell: '<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>',
  copy: '<rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
  check: '<path d="M20 6 9 17l-5-5"/>',
  checkCircle: '<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>',
  phone: '<path d="M22 16.9v3a2 2 0 0 1-2.2 2 19.8 19.8 0 0 1-8.6-3.1 19.5 19.5 0 0 1-6-6A19.8 19.8 0 0 1 2.1 4.2 2 2 0 0 1 4.1 2h3a2 2 0 0 1 2 1.7c.1.9.4 1.8.7 2.7a2 2 0 0 1-.5 2.1L8 9.8a16 16 0 0 0 6 6l1.3-1.3a2 2 0 0 1 2.1-.4c.9.3 1.8.6 2.7.7a2 2 0 0 1 1.7 2z"/>',
  message: '<path d="M21 11.5a8.4 8.4 0 0 1-9 8.4 8.6 8.6 0 0 1-3.9-.9L3 21l1.9-5.1A8.4 8.4 0 0 1 12 3a8.5 8.5 0 0 1 9 8.5z"/>',
  mail: '<rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-10 6L2 7"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7"/><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7"/>',
  external: '<path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>',
  video: '<path d="m22 8-6 4 6 4V8z"/><rect x="2" y="6" width="14" height="12" rx="2"/>',
  users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.9"/><path d="M16 3.1a4 4 0 0 1 0 7.8"/>',
  userCheck: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="m16 11 2 2 4-4"/>',
  rupee: '<path d="M6 3h12"/><path d="M6 8h12"/><path d="m6 13 8.5 8"/><path d="M6 13h3a5 5 0 0 0 0-10"/>',
  wallet: '<path d="M21 12V7H5a2 2 0 0 1 0-4h14v4"/><path d="M3 5v14a2 2 0 0 0 2 2h16v-5"/><path d="M18 12a2 2 0 0 0 0 4h4v-4z"/>',
  clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',
  calendar: '<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>',
  megaphone: '<path d="m3 11 18-5v12L3 14v-3z"/><path d="M11.6 16.8a3 3 0 1 1-5.8-1.6"/>',
  qr: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><path d="M14 14h3v3h-3zM20 14v.01M14 20h.01M17 20h4v-3"/>',
  search: '<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>',
  edit: '<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>',
  trash: '<path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>',
  upload: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m17 8-5-5-5 5"/><path d="M12 3v12"/>',
  download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="m7 10 5 5 5-5"/><path d="M12 15V3"/>',
  logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="m16 17 5-5-5-5"/><path d="M21 12H9"/>',
  refresh: '<path d="M21 12a9 9 0 0 1-15 6.7L3 16"/><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M3 21v-5h5"/><path d="M21 3v5h-5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  x: '<path d="M18 6 6 18M6 6l12 12"/>',
  alert: '<circle cx="12" cy="12" r="10"/><path d="M12 8v4M12 16h.01"/>',
  info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>',
  lock: '<rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>',
  eye: '<path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7z"/><circle cx="12" cy="12" r="3"/>',
  eyeOff: '<path d="M9.9 4.2A10 10 0 0 1 12 4c7 0 10 8 10 8a13 13 0 0 1-1.7 2.7"/><path d="M6.6 6.6A13.5 13.5 0 0 0 2 12s3 8 10 8a9.7 9.7 0 0 0 5.4-1.6"/><path d="M14.1 14.1a3 3 0 1 1-4.2-4.2"/><path d="m2 2 20 20"/>',
  book: '<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V2H6.5A2.5 2.5 0 0 0 4 4.5v15z"/><path d="M20 17v5H6.5a2.5 2.5 0 0 1 0-5"/>',
  send: '<path d="m22 2-7 20-4-9-9-4z"/><path d="M22 2 11 13"/>',
  list: '<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>',
  smartphone: '<rect x="5" y="2" width="14" height="20" rx="2"/><path d="M12 18h.01"/>',
  hourglass: '<path d="M5 22h14M5 2h14"/><path d="M17 22v-4.2a2 2 0 0 0-.6-1.4L12 12l-4.4 4.4a2 2 0 0 0-.6 1.4V22"/><path d="M7 2v4.2a2 2 0 0 0 .6 1.4L12 12l4.4-4.4a2 2 0 0 0 .6-1.4V2"/>',
  chevron: '<path d="m9 18 6-6-6-6"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
  moon: '<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/>',
  android: '<path d="M4 18a8 8 0 0 1 16 0z"/><path d="m7 11.2-2-3.2M17 11.2l2-3.2"/><path d="M9 14.5h.01M15 14.5h.01"/>',
  github: '<path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.9a3.4 3.4 0 0 0-.9-2.6c3.1-.4 6.4-1.5 6.4-7a5.4 5.4 0 0 0-1.5-3.8 5 5 0 0 0-.1-3.8s-1.2-.3-3.9 1.5a13.4 13.4 0 0 0-7 0C6.3 1.6 5.1 1.9 5.1 1.9a5 5 0 0 0-.1 3.8A5.4 5.4 0 0 0 3.5 9.5c0 5.4 3.3 6.6 6.4 7a3.4 3.4 0 0 0-.9 2.6V23"/>',
  arrowRight: '<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>',
  home: '<path d="m3 10 9-7 9 7v10a2 2 0 0 1-2 2h-4v-7H9v7H5a2 2 0 0 1-2-2z"/>',
  grid: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
  user: '<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>',
  shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>',
  menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
  key: '<circle cx="7.5" cy="15.5" r="4.5"/><path d="m21 2-9.6 9.6"/><path d="m15.5 7.5 3 3L22 7l-3-3"/>',
  sparkles: '<path d="M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9z"/><path d="M19 15l.9 2.1L22 18l-2.1.9L19 21l-.9-2.1L16 18l2.1-.9z"/>',
  graduation: '<path d="M22 10 12 5 2 10l10 5 10-5z"/><path d="M6 12v5c3 2 9 2 12 0v-5"/>',
  checkList: '<path d="m3 6 2 2 3-3"/><path d="m3 13 2 2 3-3"/><path d="M12 7h9M12 14h9M12 20h9"/>',
  settings: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 0 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 0 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 0 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 0 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>',
};

function icon(name, cls = '') {
  return `<svg class="icon ${cls}" viewBox="0 0 24 24" aria-hidden="true">${ICONS[name] || ''}</svg>`;
}

// ---------- Feedback ----------

function showToast(msg, type = 'info') {
  let wrap = document.querySelector('.toasts');
  if (!wrap) {
    wrap = document.createElement('div');
    wrap.className = 'toasts';
    wrap.setAttribute('role', 'status');
    wrap.setAttribute('aria-live', 'polite');
    document.body.appendChild(wrap);
  }
  const t = document.createElement('div');
  t.className = 'toast ' + type;
  t.innerHTML = icon(type === 'error' ? 'alert' : type === 'success' ? 'checkCircle' : 'info') + `<span>${esc(msg)}</span>`;
  wrap.appendChild(t);
  setTimeout(() => t.remove(), type === 'error' ? 5000 : 3200);
}

function showError(el, msg) {
  el.textContent = msg;
  el.classList.toggle('show', !!msg);
}

// Styled replacement for window.confirm(). Resolves true/false.
function confirmDialog({ title, message = '', confirmText = 'Confirm', danger = false }) {
  return new Promise(resolve => {
    const bg = document.createElement('div');
    bg.className = 'dialog-bg';
    bg.innerHTML = `<div class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="dlgTitle">
      <h3 id="dlgTitle">${esc(title)}</h3><p>${esc(message)}</p>
      <div class="dialog-actions">
        <button class="btn outline" data-v="0">Cancel</button>
        <button class="btn ${danger ? 'danger-solid' : 'primary'}" data-v="1">${esc(confirmText)}</button>
      </div></div>`;
    const done = v => { bg.remove(); document.removeEventListener('keydown', onKey); resolve(v); };
    const onKey = e => { if (e.key === 'Escape') done(false); };
    bg.addEventListener('click', e => {
      if (e.target === bg) done(false);
      const b = e.target.closest('[data-v]');
      if (b) done(b.dataset.v === '1');
    });
    document.addEventListener('keydown', onKey);
    document.body.appendChild(bg);
    bg.querySelector('[data-v="1"]').focus();
  });
}

// Shows a spinner on a button while an async task runs.
async function withLoading(btn, task) {
  btn.disabled = true;
  btn.classList.add('loading');
  try { return await task(); }
  finally { btn.disabled = false; btn.classList.remove('loading'); }
}

async function copyText(text, label = 'Copied') {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = document.createElement('textarea');
    ta.value = text; document.body.appendChild(ta); ta.select();
    document.execCommand('copy'); ta.remove();
  }
  showToast(label, 'success');
}

// Adds a show/hide eye button to every password field on the page.
function setupPasswordToggles() {
  document.querySelectorAll('input[type=password]').forEach(input => {
    const wrap = document.createElement('div');
    wrap.className = 'pw-wrap';
    input.parentNode.insertBefore(wrap, input);
    wrap.appendChild(input);
    const btn = document.createElement('button');
    btn.type = 'button'; btn.className = 'pw-toggle'; btn.setAttribute('aria-label', 'Show password');
    btn.innerHTML = icon('eye');
    btn.addEventListener('click', () => {
      const show = input.type === 'password';
      input.type = show ? 'text' : 'password';
      btn.innerHTML = icon(show ? 'eyeOff' : 'eye');
      btn.setAttribute('aria-label', show ? 'Hide password' : 'Show password');
    });
    wrap.appendChild(btn);
  });
}

const STATUS_LABEL = { active: 'Active', expired: 'Expired', scheduled: 'Starts later', none: 'Not paid', pending: 'Waiting' };

function badge(status, text) {
  return `<span class="badge b-${esc(status)}">${esc(text || STATUS_LABEL[status] || status)}</span>`;
}

// ---------- App shell ----------

// Current hash route, e.g. "#/enroll/3" -> { name: "enroll", params: ["3"] }.
function currentRoute() {
  const [name, ...params] = location.hash.replace(/^#\/?/, '').split('/');
  return { name: name || '', params };
}

function go(path) {
  if (location.hash === '#/' + path) window.dispatchEvent(new HashChangeEvent('hashchange'));
  else location.hash = '#/' + path;
}

// Highlights the active nav links (sidebar + mobile tab bar) for a route name.
function markNav(name) {
  document.querySelectorAll('[data-route]').forEach(a => {
    a.classList.toggle('on', a.dataset.route === name);
    if (a.dataset.route === name) a.setAttribute('aria-current', 'page'); else a.removeAttribute('aria-current');
  });
}

// Mobile: slide-in sidebar with a scrim behind it.
function setupSidebarToggle() {
  const sidebar = document.querySelector('.sidebar');
  const scrim = document.querySelector('.scrim');
  const open = v => { sidebar.classList.toggle('open', v); scrim.classList.toggle('show', v); };
  document.querySelectorAll('[data-menu]').forEach(b => b.addEventListener('click', () => open(!sidebar.classList.contains('open'))));
  scrim.addEventListener('click', () => open(false));
  sidebar.addEventListener('click', e => { if (e.target.closest('a')) open(false); });
  return open;
}

function setView(html) {
  const view = document.getElementById('view');
  view.classList.remove('view');
  void view.offsetWidth; // restart the entrance animation
  view.innerHTML = html;
  view.classList.add('view');
  window.scrollTo({ top: 0 });
}

function emptyState(ic, title, text, action = '') {
  return `<div class="card empty-state"><div class="ico">${icon(ic, 'xl')}</div><h3>${esc(title)}</h3><p>${esc(text)}</p>${action}</div>`;
}

// One of six gradients per course, so each course has its own colour.
function thumbClass(id) {
  return 't' + (Number(id) % 6);
}

// URL of a course's uploaded thumbnail ('' when none). ?v= changes when the image is replaced.
function thumbUrl(c) {
  return c.has_thumbnail ? `${API_BASE}/api/courses/${c.id}/thumbnail?v=${c.thumbnail_updated_at}` : '';
}

// Course picture: the uploaded thumbnail, or the course's gradient with an icon.
function courseThumb(c, extra = '') {
  const url = thumbUrl(c);
  return `<div class="course-thumb ${thumbClass(c.id)} ${extra}">${url ? `<img src="${url}" alt="" loading="lazy">` : icon('graduation')}</div>`;
}

// Downloads a file from the API with the login token, e.g. a receipt PDF.
async function downloadFile(path, filename) {
  const res = await fetch(API_BASE + path, { headers: authHeaders(path), credentials: 'include' });
  if (!res.ok) {
    let msg = 'Could not download the file.';
    try { msg = errorText(await res.json()) || msg; } catch { /* not JSON */ }
    const err = new Error(msg); err.status = res.status; throw err;
  }
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement('a');
  a.href = url; a.download = filename;
  document.body.appendChild(a); a.click(); a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// Fades out the logo loading screen. App pages call this after their first data is on screen;
// other pages hide it once loaded. It always shows for at least ~0.6 s so it does not flicker.
function hideSplash() {
  const splash = document.getElementById('splash');
  if (!splash || splash.classList.contains('hide')) return;
  setTimeout(() => {
    splash.classList.add('hide');
    setTimeout(() => splash.remove(), 500);
  }, Math.max(0, 600 - performance.now()));
}
(() => {
  const splash = document.getElementById('splash');
  if (!splash) return;
  if (!splash.hasAttribute('data-manual')) window.addEventListener('load', hideSplash);
  setTimeout(hideSplash, 8000); // never leave the screen covered if something fails
})();

function greeting() {
  const h = new Date().getHours();
  return h < 12 ? 'Good morning' : h < 17 ? 'Good afternoon' : 'Good evening';
}

// ---------- Light / dark theme ----------
// The saved choice is applied by a tiny script in each page's <head> (no flash on load).
// With no saved choice the site follows the device setting.

const THEME_KEY = 'gc-theme';

function currentTheme() {
  return document.documentElement.dataset.theme || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
}

function paintThemeButtons() {
  const dark = currentTheme() === 'dark';
  document.querySelectorAll('[data-theme-toggle]').forEach(btn => {
    btn.innerHTML = icon(dark ? 'sun' : 'moon') + (btn.dataset.themeToggle === 'label' ? ` ${dark ? 'Light' : 'Dark'} mode` : '');
    btn.setAttribute('aria-label', dark ? 'Switch to light theme' : 'Switch to dark theme');
    btn.title = btn.getAttribute('aria-label');
  });
}

function setTheme(theme) {
  const root = document.documentElement;
  root.classList.add('theme-anim');
  root.dataset.theme = theme;
  try { localStorage.setItem(THEME_KEY, theme); } catch { /* private mode: still switches for this page */ }
  paintThemeButtons();
  setTimeout(() => root.classList.remove('theme-anim'), 450);
}

document.querySelectorAll('[data-theme-toggle]').forEach(btn =>
  btn.addEventListener('click', () => setTheme(currentTheme() === 'dark' ? 'light' : 'dark')));
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', paintThemeButtons);
paintThemeButtons();
