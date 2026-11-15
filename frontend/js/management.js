// Management team app: Home, Courses, Updates and Profile. Members see courses and prices,
// updates the admin sends the team, and share their Management ID with students; they cannot enroll.

let data = null;    // /api/management/me response
let updates = null; // /api/management/me/announcements response
let seenMarked = false;
let shownSeenAt = null; // keeps NEW tags while Updates stays open after it marked them read

const VIEWS = { home: viewHome, courses: viewCourses, updates: viewUpdates, profile: viewProfile };

// ---------- Data & router ----------

async function load() {
  try {
    [data, updates] = await Promise.all([api('/api/management/me'), api('/api/management/me/announcements')]);
  } catch (e) {
    if (e.status === 401) { location.replace('management-login.html'); return false; }
    showToast(e.message, 'error');
    return false;
  }
  const m = data.manager;
  for (const id of ['sideAvatar', 'topAvatar']) document.getElementById(id).textContent = initials(m.name);
  document.getElementById('sideName').textContent = m.name;
  document.getElementById('sideCode').textContent = 'Management ID ' + m.manager_code;
  renderUnread();
  return true;
}

function renderUnread() {
  const n = data.unread_announcements;
  const nav = document.getElementById('navUnread');
  nav.textContent = n; nav.hidden = !n;
  document.getElementById('bellBtn').innerHTML = icon('bell') + (n ? `<span class="dot-count">${n > 9 ? '9+' : n}</span>` : '');
  const tab = document.querySelector('.tabbar [data-route=updates]');
  tab.querySelector('.dot')?.remove();
  if (n) tab.insertAdjacentHTML('beforeend', '<span class="dot"></span>');
}

async function route() {
  let { name } = currentRoute();
  if (!VIEWS[name]) { name = 'home'; history.replaceState(null, '', '#/home'); }
  markNav(name);
  if (name !== 'updates') shownSeenAt = null;
  if (!data) {
    if (await load()) VIEWS[name]();
    hideSplash();
    return;
  }
  VIEWS[name]();
  // Pick up new updates (and a revoked login) on every page switch.
  const before = JSON.stringify([data, updates]);
  if (await load() && JSON.stringify([data, updates]) !== before && currentRoute().name === name && name !== 'profile') VIEWS[name]();
}

// ---------- Shared pieces ----------

function durationLabel(days) {
  if (days % 365 === 0) return (days / 365) + (days === 365 ? ' year' : ' years');
  if (days % 30 === 0) return (days / 30) + (days === 30 ? ' month' : ' months');
  return days + ' days';
}

function idChip(m) {
  return `<span class="chip">Management ID <b class="mono">${esc(m.manager_code)}</b> <button type="button" data-copy="${esc(m.manager_code)}">Copy</button></span>`;
}

function wireCopy(root = document) {
  root.querySelectorAll('[data-copy]').forEach(b => b.addEventListener('click', () => copyText(b.dataset.copy, 'Management ID copied')));
}

function courseCard(c) {
  return `<article class="card course-card">
    ${courseThumb(c)}
    <h3>${esc(c.title)}</h3>
    ${c.description ? `<p class="desc">${esc(c.description)}</p>` : ''}
    ${c.features.length ? `<ul class="mini-features">${c.features.slice(0, 4).map(f => `<li>${icon('check')} ${esc(f)}</li>`).join('')}</ul>` : ''}
    <div class="foot"><div class="small"><span class="price-tag">${rupees(c.price)}</span> <small class="muted">/ ${durationLabel(c.default_days)}</small></div></div>
  </article>`;
}

// ---------- Home ----------

function viewHome() {
  const m = data.manager;
  const steps = [
    ['users', 'Tell students about G-CATION', 'Share the courses and prices you see here.'],
    ['send', 'Give them your Management ID', `They type ${m.manager_code} as the Referral ID when they send their payment details.`],
    ['checkCircle', 'Payment is verified', 'The admin sees your ID on their payment and their receipt.'],
  ];
  setView(`
    <section class="hero-card on-hero">
      <span class="kicker">${greeting()}</span>
      <h2>Hello, ${esc(m.name.split(' ')[0])}</h2>
      <p>Share your Management ID with students. They enter it as the <b>Referral ID</b> when they pay.</p>
      <div class="row" style="margin-top:16px">${idChip(m)}<a class="btn white sm" href="#/courses">${icon('graduation')} View courses</a></div>
    </section>

    <div style="margin-top:26px">
      <div class="section-title"><h2>How referrals work</h2></div>
      <div class="grid-3">${steps.map(([ic, title, text], i) => `<div class="card">
        <span class="stat-ico">${icon(ic)}</span>
        <h3 style="margin:12px 0 4px;font-size:1.02rem">${i + 1}. ${esc(title)}</h3>
        <p class="muted small" style="margin:0">${esc(text)}</p></div>`).join('')}</div>
    </div>

    ${updates.items.length ? `<div style="margin-top:26px">
      <div class="section-title"><h2>Latest updates</h2><a href="#/updates">See all →</a></div>
      ${updates.items.slice(0, 2).map(u => updateCard(u, updates.seen_at)).join('')}
    </div>` : ''}

    <div style="margin-top:26px">
      <div class="section-title"><h2>Courses</h2><a href="#/courses">All courses →</a></div>
      ${data.courses.length ? `<div class="grid-3">${data.courses.slice(0, 3).map(courseCard).join('')}</div>`
        : emptyState('sparkles', 'No courses open right now', 'New courses will show up here.')}
    </div>
  `);
  wireCopy();
}

// ---------- Courses ----------

function viewCourses() {
  setView(`
    <div class="view-head"><div><span class="kicker">Courses</span><h1>Courses and prices</h1><p>Everything students can join right now.</p></div></div>
    <div class="notice info" style="margin-bottom:16px">${icon('info')}<div>Management accounts cannot enroll. Students join from their own account with <a href="login.html#register" style="font-weight:700">Enroll now</a>.</div></div>
    ${data.courses.length ? `<div class="grid-3">${data.courses.map(courseCard).join('')}</div>`
      : emptyState('sparkles', 'No courses open right now', 'New courses will show up here.')}
  `);
}

// ---------- Updates ----------

function updateIcon(u) {
  const url = u.link_url || '';
  if (url.includes('meet.google') || url.includes('zoom.us')) return 'video';
  if (url.includes('whatsapp') || url.includes('t.me') || url.includes('telegram')) return 'message';
  return url ? 'link' : 'megaphone';
}

function updateCard(u, seenAt) {
  const unread = u.created_at > seenAt;
  return `<article class="card update ${unread ? 'unread' : ''}">
    <div class="update-icon">${icon(updateIcon(u), 'lg')}</div>
    <div style="min-width:0">
      <h3>${esc(u.title)} ${unread ? '<span class="badge b-new">NEW</span>' : ''}</h3>
      <div class="meta">${timeAgo(u.created_at)}${u.updated_at > u.created_at + 60 ? ' · edited' : ''}</div>
      ${u.message ? `<p class="msg">${linkify(u.message)}</p>` : ''}
      ${u.link_url ? `<a class="btn primary sm" href="${esc(u.link_url)}" target="_blank" rel="noopener">${icon('external')} ${esc(u.link_label || 'Open link')}</a>` : ''}
    </div>
  </article>`;
}

function viewUpdates() {
  setView(`
    <div class="view-head"><div><span class="kicker">Updates</span><h1>From G-CATION</h1><p>Messages, meeting links and instructions for the management team.</p></div></div>
    ${updates.items.length ? updates.items.map(u => updateCard(u, shownSeenAt ?? updates.seen_at)).join('')
      : emptyState('megaphone', 'No updates yet', 'Messages from the G-CATION admin for the team will appear here.')}
  `);
  // Opening Updates marks everything as read.
  if (seenMarked || !data.unread_announcements) return;
  seenMarked = true;
  shownSeenAt = updates.seen_at;
  api('/api/management/me/announcements/seen', { method: 'POST' }).then(() => {
    data.unread_announcements = 0;
    seenMarked = false;
    renderUnread();
  }).catch(() => { seenMarked = false; });
}

// ---------- Profile ----------

function viewProfile() {
  const m = data.manager;
  setView(`
    <div class="view-head"><div><span class="kicker">Profile</span><h1>Your account</h1><p>Your details and password.</p></div></div>
    <div class="grid-2" style="align-items:start">
      <div class="stack">
        <div class="card pad-lg">
          <div class="row" style="gap:16px">
            <span class="avatar-sm avatar-lg">${esc(initials(m.name))}</span>
            <div style="min-width:0"><h2 style="margin:0;font-size:1.25rem">${esc(m.name)}</h2><div class="muted small">${esc(m.email)}</div>
              <div class="tiny faint">Team member since ${fmtDate(m.created_at)}</div></div>
          </div>
          <div class="row" style="margin-top:16px">${idChip(m)}</div>
        </div>
        <div class="card pad-lg">
          <h3 class="card-title">${icon('user')} Personal details</h3>
          <dl class="sent-list" style="margin-top:12px">
            <dt>Name</dt><dd>${esc(m.name)}</dd><dt>Email</dt><dd>${esc(m.email)}</dd>
            ${m.phone ? `<dt>Phone</dt><dd>${esc(m.phone)}</dd>` : ''}
          </dl>
          <p class="tiny muted" style="margin:12px 0 0">To change these details, contact the G-CATION admin.</p>
        </div>
      </div>
      <div class="stack">
        <form class="card pad-lg" id="passwordForm" novalidate>
          <h3 class="card-title">${icon('key')} Change password</h3>
          <p class="card-sub">You stay logged in here. Other devices are logged out.</p>
          <div class="field"><label for="pCur">Current password</label><input id="pCur" name="current_password" type="password" autocomplete="current-password" required></div>
          <div class="field"><label for="pNew">New password</label><input id="pNew" name="new_password" type="password" autocomplete="new-password" minlength="8" required><small>At least 8 characters.</small></div>
          <div class="error"></div>
          <button class="btn primary" type="submit">${icon('key')} Update password</button>
        </form>
        <button class="btn danger block page-logout" id="logoutBtn2">${icon('logout')} Log out</button>
      </div>
    </div>
  `);
  wireCopy();
  setupPasswordToggles();
  document.getElementById('logoutBtn2').addEventListener('click', logout);
  const pw = document.getElementById('passwordForm');
  pw.addEventListener('submit', async e => {
    e.preventDefault();
    const err = pw.querySelector('.error');
    if (pw.new_password.value.length < 8) { showError(err, 'New password must be at least 8 characters.'); return; }
    showError(err, '');
    try {
      await withLoading(pw.querySelector('button[type=submit]'), () => api('/api/management/me/password', { method: 'POST', body: {
        current_password: pw.current_password.value, new_password: pw.new_password.value,
      }}));
      pw.reset();
      showToast('Password updated.', 'success');
    } catch (ex) { showError(err, ex.message); }
  });
}

// ---------- Boot ----------

async function logout() {
  await api('/api/management/logout', { method: 'POST' }).catch(() => {});
  setToken('management', '');
  location.href = 'management-login.html';
}

document.querySelectorAll('[data-icon]').forEach(el => el.insertAdjacentHTML('afterbegin', icon(el.dataset.icon)));
document.getElementById('logoutBtn').innerHTML = icon('logout') + ' Log out';
document.getElementById('logoutBtn').addEventListener('click', logout);
window.addEventListener('hashchange', route);
route();
