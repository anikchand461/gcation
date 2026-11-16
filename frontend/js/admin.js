// Admin app: Overview, Students, Payment notes, Payments, Management, Updates, Payment QR and Courses.

const DAY = 86400;
const PRESETS = [[30, '1 month'], [90, '3 months'], [180, '6 months'], [365, '1 year']];
const VIEWS = { overview: viewOverview, students: viewStudents, notes: viewNotes, payments: viewPayments, management: viewManagement, updates: viewUpdates, qr: viewQr, courses: viewCourses, account: viewAccount };

let students = [];
let studentFilter = 'all';
let studentQuery = '';
let payments = [];
let courses = [];
let audience = {};
let editingAnn = null;
let current = null; // student shown in the drawer

const nowTs = () => Math.floor(Date.now() / 1000);

async function guarded(fn) {
  try { return await fn(); }
  catch (e) {
    if (e.status === 401) { location.replace('admin-login.html'); return new Promise(() => {}); }
    showToast(e.message, 'error');
    throw e;
  }
}

// ---------- Router & chrome ----------

function route() {
  let { name } = currentRoute();
  if (!VIEWS[name]) { name = 'overview'; history.replaceState(null, '', '#/overview'); }
  markNav(name);
  VIEWS[name]().catch(() => {});
}

function setPendingCount(n) {
  const nav = document.getElementById('navNotes');
  nav.textContent = n; nav.hidden = !n;
  document.getElementById('topNotes').innerHTML = icon('hourglass') + (n ? `<span class="dot-count">${n}</span>` : '');
}

async function refreshCounts() {
  const o = await api('/api/admin/overview').catch(() => null);
  if (o) setPendingCount(o.pending_claims);
  return o;
}

function head(kicker, title, sub = '', actions = '') {
  return `<div class="view-head"><div><span class="kicker">${kicker}</span><h1>${title}</h1>${sub ? `<p>${sub}</p>` : ''}</div>${actions}</div>`;
}

const skeleton = (h = 300) => `<div class="skeleton" style="height:${h}px"></div>`;

// "GCS001", or a note that the student has no Student ID yet (they get one when a payment is approved).
function studentIdText(code) {
  return code ? esc(code) : 'No Student ID yet';
}

// Referrer of a payment note: name and Management ID.
function referralText(c) {
  return [c.referred_by, c.referral_id && `${c.referral_id}${c.referral_name ? ' (' + c.referral_name + ')' : ''}`].filter(Boolean).join(' · ');
}

// Opens the payment screenshot a student uploaded (kept until 5 minutes after the payment is approved).
function showScreenshot(claimId) {
  const bg = document.createElement('div');
  bg.className = 'dialog-bg';
  bg.innerHTML = `<div class="dialog shot-dialog" role="dialog" aria-modal="true" aria-label="Payment screenshot">
    <img data-auth-src="/api/admin/claims/${claimId}/screenshot?v=${Date.now()}" alt="Payment screenshot">
    <div class="dialog-actions"><button class="btn primary" data-close>Close</button></div></div>`;
  const close = () => { bg.remove(); document.removeEventListener('keydown', onKey); };
  const onKey = e => { if (e.key === 'Escape') { e.stopPropagation(); close(); } };
  bg.addEventListener('click', e => { if (e.target === bg || e.target.closest('[data-close]')) close(); });
  document.addEventListener('keydown', onKey, true);
  document.body.appendChild(bg);
}

function shotButton(c, cls = 'outline') {
  return c.has_screenshot ? `<button type="button" class="btn ${cls} sm" data-shot="${c.id}">${icon('eye')} Screenshot</button>` : '';
}

function wireShots(root) {
  root.querySelectorAll('[data-shot]').forEach(b => b.addEventListener('click', ev => { ev.stopPropagation(); showScreenshot(b.dataset.shot); }));
}

function qrLabel(slot) {
  return slot === 'primary' ? 'Primary QR' : slot === 'secondary' ? 'Secondary QR' : 'QR not specified';
}

// ---------- Overview ----------

async function viewOverview() {
  setView(head(new Date().toLocaleDateString('en-IN', { weekday: 'long', day: 'numeric', month: 'long' }), greeting()) + skeleton(420));
  const [o, claims, studentList, pays] = await guarded(() => Promise.all([
    api('/api/admin/overview'), api('/api/admin/claims?status=pending'), api('/api/admin/students'), api('/api/admin/payments'),
  ]));
  setPendingCount(o.pending_claims);
  students = studentList;
  payments = pays;
  const t = nowTs();
  const expiring = studentList
    .map(s => ({ s, a: s.access.filter(a => a.status === 'active' && a.until - t <= 7 * DAY).sort((x, y) => x.until - y.until)[0] }))
    .filter(x => x.a);

  const tiles = [
    ['users', '', 'Students', o.students, 'students'],
    ['userCheck', 'g', 'Active now', o.active_students, 'students/active'],
    ['hourglass', 'a', 'Payment notes', o.pending_claims, 'notes'],
    ['clock', 'r', 'Expiring in 7 days', o.expiring_soon, 'students/expiring'],
    ['wallet', 'b', 'This month', rupees(o.collected_this_month), 'payments'],
    ['rupee', '', 'Total collected', rupees(o.total_collected), 'payments'],
  ];

  setView(`
    ${head(new Date().toLocaleDateString('en-IN', { weekday: 'long', day: 'numeric', month: 'long' }), greeting(), 'Here is what needs your attention today.',
      `<div class="row"><a class="btn outline sm" href="#/updates">${icon('megaphone')} Post update</a><a class="btn primary sm" href="#/students">${icon('users')} Students</a></div>`)}
    <div class="stats">${tiles.map(([ic, tone, label, value, to]) =>
      `<button class="stat-tile" data-go="${to}"><span class="stat-ico ${tone}">${icon(ic)}</span><span><small>${label}</small><b>${value}</b></span></button>`).join('')}</div>

    <div class="grid-2" style="margin-top:20px;align-items:start">
      <div class="card">
        <div class="section-title"><h2>${icon('hourglass')} Payments to verify</h2><a href="#/notes">All →</a></div>
        ${claims.length ? claims.slice(0, 5).map(c => `
          <div class="list-row"><span class="avatar-sm">${esc(initials(c.student_name))}</span>
            <div class="grow"><b>${esc(c.student_name)} · ${c.amount != null ? rupees(c.amount) : '?'}</b>
              <span>${esc(qrLabel(c.qr_slot))}${c.method ? ' · ' + esc(c.method) : ''} · ${timeAgo(c.created_at)}</span></div>
            <button class="btn success sm" data-verify="${c.student_id}" data-claim="${c.id}">${icon('check')} Verify</button></div>`).join('')
          : `<div class="empty-cell">${icon('checkCircle')}<div>All caught up. Nothing to verify.</div></div>`}
      </div>
      <div class="card">
        <div class="section-title"><h2>${icon('clock')} Expiring soon</h2><a href="#/students/expiring">All →</a></div>
        ${expiring.length ? expiring.slice(0, 5).map(({ s, a }) => `
          <div class="list-row"><span class="avatar-sm">${esc(initials(s.name))}</span>
            <div class="grow"><b>${esc(s.name)}</b><span>${esc(a.title)} · ${a.days_left} day${a.days_left === 1 ? '' : 's'} left</span></div>
            ${s.phone ? `<a class="btn outline sm icon-only" href="${esc(waHref(s.phone, `Hi ${s.name.split(' ')[0]}, your G-CATION course "${a.title}" ends on ${fmtDate(a.until)}. Renew to keep learning!`))}" target="_blank" rel="noopener" aria-label="WhatsApp">${icon('message')}</a>` : ''}
            <button class="btn outline sm" data-open="${s.id}">Open</button></div>`).join('')
          : `<div class="empty-cell">${icon('calendar')}<div>No access ends in the next 7 days.</div></div>`}
      </div>
    </div>

    <div class="card" style="margin-top:18px">
      <div class="section-title"><h2>${icon('wallet')} Recent payments</h2><a href="#/payments">All →</a></div>
      ${pays.length ? pays.slice(0, 5).map(p => `
        <div class="list-row"><span class="avatar-sm">${esc(initials(p.student_name))}</span>
          <div class="grow"><b>${esc(p.student_name)}</b><span>${esc(p.course_title)} · ${fmtDate(p.granted_at)}</span></div>
          <b>${rupees(p.amount_paid)}</b></div>`).join('')
        : `<div class="empty-cell">${icon('wallet')}<div>No payments recorded yet.</div></div>`}
    </div>
  `);
  const view = document.getElementById('view');
  view.querySelectorAll('[data-go]').forEach(b => b.addEventListener('click', () => go(b.dataset.go)));
  view.querySelectorAll('[data-verify]').forEach(b => b.addEventListener('click', () => openStudent(Number(b.dataset.verify), Number(b.dataset.claim))));
  view.querySelectorAll('[data-open]').forEach(b => b.addEventListener('click', () => openStudent(Number(b.dataset.open))));
}

// ---------- Students ----------

const FILTERS = [['all', 'All'], ['active', 'Active'], ['waiting', 'Waiting'], ['expiring', 'Expiring soon'], ['expired', 'Expired'], ['none', 'Not paid']];

function studentState(s) {
  const act = s.access.filter(a => a.status === 'active');
  if (act.length) return Math.min(...act.map(a => a.days_left)) <= 7 ? 'expiring' : 'active';
  if (s.pending_claims) return 'waiting';
  if (s.access.some(a => a.status === 'scheduled')) return 'active';
  if (s.access.some(a => a.status === 'expired')) return 'expired';
  return 'none';
}
const matchesFilter = (s, f) => f === 'all' || (f === 'active' ? ['active', 'expiring'].includes(studentState(s)) : studentState(s) === f);

function statusBadge(s) {
  const st = studentState(s);
  const soonest = s.access.filter(a => a.status === 'active').sort((a, b) => a.days_left - b.days_left)[0];
  if (st === 'expiring') return badge('pending', `Expires in ${soonest.days_left}d`);
  if (st === 'active') return badge('active');
  if (st === 'waiting') return badge('pending', 'Payment note');
  if (st === 'expired') return badge('expired');
  return badge('none');
}

async function viewStudents() {
  const sub = currentRoute().params[0];
  if (sub && FILTERS.some(([f]) => f === sub)) studentFilter = sub;
  setView(`
    ${head('People', 'Students', 'Everyone who registered. Click a student to verify payment or manage access.')}
    <div class="toolbar"><div class="search">${icon('search')}<input id="search" type="search" placeholder="Search name, email, phone or Student ID" aria-label="Search students" value="${esc(studentQuery)}"></div></div>
    <div class="filters" id="filters"></div>
    <div class="card table-wrap"><table class="responsive">
      <thead><tr><th>Student</th><th>Phone</th><th>Status</th><th>Access until</th><th>Joined</th><th></th></tr></thead>
      <tbody id="studentRows"><tr><td colspan="6">${skeleton(120)}</td></tr></tbody>
    </table></div>
  `);
  let timer;
  document.getElementById('search').addEventListener('input', e => {
    studentQuery = e.target.value;
    clearTimeout(timer);
    timer = setTimeout(loadStudents, 250);
  });
  await loadStudents();
}

async function loadStudents() {
  students = await guarded(() => api('/api/admin/students?q=' + encodeURIComponent(studentQuery)));
  renderStudents();
}

function renderStudents() {
  const filters = document.getElementById('filters');
  if (!filters) return;
  filters.innerHTML = FILTERS.map(([f, label]) =>
    `<button class="filter ${f === studentFilter ? 'on' : ''}" data-f="${f}">${label}<b>${students.filter(s => matchesFilter(s, f)).length}</b></button>`).join('');
  filters.querySelectorAll('.filter').forEach(b => b.addEventListener('click', () => { studentFilter = b.dataset.f; renderStudents(); }));

  const rows = students.filter(s => matchesFilter(s, studentFilter));
  const body = document.getElementById('studentRows');
  if (!rows.length) {
    body.innerHTML = `<tr><td colspan="6" class="empty-cell">${icon('users')}<div>${studentQuery ? 'No students match your search.' : students.length ? 'No students in this group.' : 'No students have registered yet.'}</div></td></tr>`;
    return;
  }
  body.innerHTML = rows.map(s => {
    const until = s.access.filter(a => a.until && a.status !== 'expired').map(a => a.until);
    const ended = s.access.map(a => a.until).filter(Boolean);
    return `<tr class="click" data-id="${s.id}">
      <td class="primary-cell"><div class="who"><span class="avatar-sm">${esc(initials(s.name))}</span><div style="min-width:0"><b>${esc(s.name)}</b><span>${s.student_code ? esc(s.student_code) + ' · ' : ''}${esc(s.email)}</span></div></div></td>
      <td data-label="Phone">${s.phone ? `<a href="${esc(telHref(s.phone))}" class="strong" data-stop>${esc(s.phone)}</a>` : '<span class="faint">—</span>'}</td>
      <td data-label="Status">${statusBadge(s)}</td>
      <td data-label="Access until">${until.length ? fmtDate(Math.max(...until)) : ended.length ? `<span class="muted">ended ${fmtDate(Math.max(...ended))}</span>` : '<span class="faint">—</span>'}</td>
      <td data-label="Joined" class="muted hide-sm">${fmtDate(s.created_at)}</td>
      <td class="right hide-sm">${icon('chevron')}</td>
    </tr>`;
  }).join('');
  body.querySelectorAll('[data-stop]').forEach(a => a.addEventListener('click', e => e.stopPropagation()));
  body.querySelectorAll('tr.click').forEach(tr => tr.addEventListener('click', () => openStudent(Number(tr.dataset.id))));
}

// ---------- Student drawer ----------

async function openStudent(id, claimId = null) {
  current = await guarded(() => api('/api/admin/students/' + id));
  current.claimId = claimId;
  renderDrawer();
  document.getElementById('drawer').classList.add('show');
  document.getElementById('drawerBg').classList.add('show');
}

function closeDrawer() {
  document.getElementById('drawer').classList.remove('show');
  document.getElementById('drawerBg').classList.remove('show');
  current = null;
}
document.getElementById('drawerBg').addEventListener('click', closeDrawer);
document.addEventListener('keydown', e => { if (e.key === 'Escape' && current && !document.querySelector('.dialog-bg')) closeDrawer(); });

// After a change in the drawer, refresh the drawer, the counts and the current page.
async function afterChange() {
  await openStudent(current.student.id);
  refreshCounts();
  const { name } = currentRoute();
  if (name === 'students') loadStudents().catch(() => {});
  else if (VIEWS[name] && name !== 'updates' && name !== 'qr' && name !== 'courses') VIEWS[name]().catch(() => {});
}

function renderDrawer() {
  const { student: s, courses: cs, enrollments, claims } = current;
  const pendingClaims = claims.filter(c => c.status === 'pending');
  const t = nowTs();

  document.getElementById('drawerHead').innerHTML = `
    <span class="avatar-sm" style="width:50px;height:50px;font-size:1rem;border-radius:15px">${esc(initials(s.name))}</span>
    <div style="flex:1;min-width:0">
      <h2 id="drawerName">${esc(s.name)}</h2>
      <div class="muted small">${studentIdText(s.student_code)} · ${esc(s.email)}</div>
      <div class="contact-actions">
        ${s.phone ? `<a class="btn outline sm" href="${esc(telHref(s.phone))}">${icon('phone')} Call</a>
          <a class="btn outline sm" href="${esc(waHref(s.phone, `Hi ${s.name.split(' ')[0]}, this is G-CATION.`))}" target="_blank" rel="noopener">${icon('message')} WhatsApp</a>` : ''}
        <a class="btn outline sm" href="mailto:${esc(s.email)}">${icon('mail')} Email</a>
      </div>
    </div>
    <button class="icon-btn" aria-label="Close" id="drawerClose">${icon('x')}</button>`;
  document.getElementById('drawerClose').addEventListener('click', closeDrawer);

  document.getElementById('drawerBody').innerHTML = `
    <h3>Course access</h3>
    ${cs.map(c => `<div class="card access-line">
      <div><b>${esc(c.title)}</b><div class="muted small">${
        c.access.status === 'active' ? `Until ${fmtDate(c.access.until)} · ${c.access.days_left} days left`
        : c.access.status === 'expired' ? `Ended ${fmtDate(c.access.until)}`
        : c.access.status === 'scheduled' ? `Scheduled until ${fmtDate(c.access.until)}` : 'No access yet'}</div></div>
      ${badge(c.access.status)}</div>`).join('')}

    ${pendingClaims.length ? `<h3>Payment notes from student</h3>${pendingClaims.map(c => `
      <div class="notice warn" style="margin-bottom:8px">${icon('hourglass')}<div>
        <b>${c.amount != null ? rupees(c.amount) : 'Amount not given'}</b> to ${esc(qrLabel(c.qr_slot))}
        ${c.method ? ' via ' + esc(c.method) : ''}${c.payer_name ? ' · payer <b>' + esc(c.payer_name) + '</b>' : ''}
        ${c.paid_on ? ' · ' + esc(c.paid_on) : ''}${c.reference ? ' · UTR <span class="mono">' + esc(c.reference) + '</span>' : ''}
        ${c.age ? ' · age ' + c.age : ''}${referralText(c) ? ' · referred by <b>' + esc(referralText(c)) + '</b>' : ''}
        <div class="tiny" style="margin-top:2px">${esc(c.course_title)} · sent ${fmtDateTime(c.created_at)}</div>
        ${c.has_screenshot ? `<div style="margin-top:6px">${shotButton(c)}</div>` : ''}</div></div>`).join('')}` : ''}

    <h3>Give course access</h3>
    <form class="card" id="grantForm" novalidate>
      <p class="muted small" style="margin:0 0 4px">Do this after you have confirmed the payment in your bank account.</p>
      <div class="row2">
        <div class="field"><label for="gCourse">Course</label>
          <select id="gCourse" name="course_id">${cs.map(c => `<option value="${c.id}">${esc(c.title)}</option>`).join('')}</select></div>
        <div class="field"><label for="gAmount">Amount received (₹)</label><input id="gAmount" name="amount" type="number" min="0" step="0.01" required></div>
      </div>
      <div class="field"><span class="label">Access period</span>
        <div class="presets">${PRESETS.map(([d, l]) => `<button type="button" data-days="${d}">${l}</button>`).join('')}</div>
        <div class="row" style="margin-top:6px"><input name="days" type="number" min="1" max="3650" required class="input" style="max-width:110px" aria-label="Days"> <span class="muted small">days</span></div>
      </div>
      <div class="field"><label for="gStart">Start</label>
        <select id="gStart" name="start">
          <option value="after_current">After current access ends (or today if none)</option>
          <option value="now">From today</option>
        </select></div>
      ${pendingClaims.length ? `<div class="field"><label for="gClaim">Matches payment note</label>
        <select id="gClaim" name="claim_id"><option value="">None</option>${pendingClaims.map(c =>
          `<option value="${c.id}">${fmtDate(c.created_at)} · ${esc(c.payer_name || 'no name')} · ${c.amount != null ? rupees(c.amount) : '?'}</option>`).join('')}</select></div>` : ''}
      <div class="field"><label for="gNote">Payment notes</label><textarea id="gNote" name="note" maxlength="500" placeholder="e.g. SBI, paid by Rahul Das, UTR 4123…, confirmed on call"></textarea></div>
      <div class="grant-preview" id="grantPreview"></div>
      <div class="error" id="grantErr"></div>
      <button class="btn success block lg" type="submit">${icon('checkCircle')} Give access</button>
    </form>

    <h3>Access &amp; payment history</h3>
    ${enrollments.length ? `<div class="card table-wrap"><table class="responsive">
      <thead><tr><th>Given</th><th>Course</th><th class="num">Amount</th><th>Period</th><th></th></tr></thead>
      <tbody>${enrollments.map(e => `<tr class="${e.revoked_at ? 'revoked' : ''}">
        <td class="primary-cell"><b>${esc(e.course_title)}</b><div class="tiny muted">Given ${fmtDate(e.granted_at)}${e.note ? ' · ' + esc(e.note) : ''}</div></td>
        <td data-label="Course" class="hide-sm">${esc(e.course_title)}</td>
        <td data-label="Amount" class="num">${rupees(e.amount_paid)}</td>
        <td data-label="Period" class="small"><span class="strike">${fmtDate(e.start_at)} → ${fmtDate(e.end_at)}</span></td>
        <td class="right actions" style="white-space:nowrap">${adminReceiptButton(e)}
          ${e.revoked_at ? `<span class="tiny">Revoked ${fmtDate(e.revoked_at)}</span>`
          : e.end_at > t ? `<button class="btn danger sm" data-revoke="${e.id}">Revoke</button>` : ''}</td></tr>`).join('')}</tbody>
    </table></div>` : '<p class="muted small">No access given yet.</p>'}

    <h3>Account</h3>
    <details class="fold"><summary>${icon('edit')} Edit student details</summary><div class="fold-body">
      <form id="editForm" novalidate>
        <div class="field"><label for="eName">Full name</label><input id="eName" name="name" maxlength="80" value="${esc(s.name)}"></div>
        <div class="row2">
          <div class="field"><label for="eEmail">Email</label><input id="eEmail" name="email" type="email" maxlength="120" value="${esc(s.email)}"></div>
          <div class="field"><label for="ePhone">Phone</label><input id="ePhone" name="phone" maxlength="20" value="${esc(s.phone)}"></div>
        </div>
        <div class="error"></div>
        <button class="btn primary sm" type="submit">${icon('check')} Save details</button>
      </form></div></details>
    <details class="fold"><summary>${icon('key')} Reset password</summary><div class="fold-body">
      <form id="resetForm" novalidate>
        <p class="muted small" style="margin:0 0 6px">For students who forgot their password. Set a new one, tell the student, and they are logged out everywhere.</p>
        <div class="row" style="align-items:flex-end">
          <div class="field" style="flex:1;margin:6px 0"><label for="rPw">New password</label><input id="rPw" name="new_password" minlength="8" autocomplete="off"></div>
          <button type="button" class="btn outline sm" id="genPw" style="margin-bottom:8px">${icon('sparkles')} Generate</button>
        </div>
        <div class="error"></div>
        <button class="btn primary sm" type="submit">${icon('key')} Set new password</button>
      </form></div></details>
  `;
  wireGrantForm(pendingClaims);
  wireAccountForms();
  document.querySelectorAll('[data-revoke]').forEach(b => b.addEventListener('click', () => revoke(Number(b.dataset.revoke))));
  wireAdminReceipts(document.getElementById('drawerBody'));
  wireShots(document.getElementById('drawerBody'));
}

function adminReceiptButton(e) {
  return `<button type="button" class="btn outline sm" data-receipt="${e.id}" data-no="${esc(e.receipt_no)}" title="Download receipt ${esc(e.receipt_no)}">${icon('download')} Receipt</button>`;
}

function wireAdminReceipts(root) {
  root.querySelectorAll('[data-receipt]').forEach(b => b.addEventListener('click', ev => {
    ev.stopPropagation();
    withLoading(b, () => downloadFile(`/api/admin/enrollments/${b.dataset.receipt}/receipt.pdf`, `G-CATION-receipt-${b.dataset.no}.pdf`))
      .catch(e => showToast(e.message, 'error'));
  }));
}

function wireGrantForm(pendingClaims) {
  const form = document.getElementById('grantForm');
  const courseById = id => current.courses.find(c => c.id === Number(id));
  const updatePreview = () => {
    const c = courseById(form.course_id.value);
    const days = Number(form.days.value);
    form.querySelectorAll('.presets button').forEach(b => b.classList.toggle('on', Number(b.dataset.days) === days));
    const box = document.getElementById('grantPreview');
    if (!days) { box.innerHTML = `${icon('calendar')} Choose how many days.`; return; }
    const continues = form.start.value === 'after_current' && ['active', 'scheduled'].includes(c.access.status);
    const start = continues ? c.access.until : nowTs();
    box.innerHTML = `${icon('calendar')}<div>Access: <b>${fmtDate(start)} → ${fmtDate(start + days * DAY)}</b>${continues ? '<div class="tiny muted">Added after the current access ends.</div>' : ''}</div>`;
  };
  const setDays = d => { form.days.value = d; updatePreview(); };
  const fillFromCourse = () => {
    const c = courseById(form.course_id.value);
    form.amount.value = Number(c.price);
    setDays(c.default_days);
  };
  form.course_id.addEventListener('change', fillFromCourse);
  form.days.addEventListener('input', updatePreview);
  form.start.addEventListener('change', updatePreview);
  form.querySelectorAll('.presets button').forEach(b => b.addEventListener('click', () => setDays(b.dataset.days)));
  if (form.claim_id) {
    form.claim_id.addEventListener('change', () => {
      const claim = pendingClaims.find(c => c.id === Number(form.claim_id.value));
      if (!claim) return;
      form.course_id.value = claim.course_id;
      fillFromCourse();
      if (claim.amount != null) form.amount.value = Number(claim.amount);
      form.note.value = [qrLabel(claim.qr_slot), claim.method, claim.payer_name && 'paid by ' + claim.payer_name,
        claim.reference && 'UTR ' + claim.reference, claim.paid_on && 'on ' + claim.paid_on,
        claim.referral_id && 'referral ' + claim.referral_id].filter(Boolean).join(', ');
    });
  }
  fillFromCourse();
  if (form.claim_id) {
    form.claim_id.value = current.claimId || (pendingClaims.length === 1 ? pendingClaims[0].id : '');
    form.claim_id.dispatchEvent(new Event('change'));
  }
  form.addEventListener('submit', grantAccess);
}

function wireAccountForms() {
  const ef = document.getElementById('editForm');
  ef.addEventListener('submit', async e => {
    e.preventDefault();
    const err = ef.querySelector('.error');
    showError(err, '');
    try {
      await withLoading(ef.querySelector('button[type=submit]'), () => api(`/api/admin/students/${current.student.id}`, { method: 'PUT', body: {
        name: ef.name.value, email: ef.email.value, phone: ef.phone.value,
      }}));
      showToast('Student details saved.', 'success');
      afterChange();
    } catch (ex) { showError(err, ex.message); }
  });

  const rf = document.getElementById('resetForm');
  document.getElementById('genPw').addEventListener('click', () => {
    const chars = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    const bytes = crypto.getRandomValues(new Uint32Array(10));
    rf.new_password.value = Array.from(bytes, b => chars[b % chars.length]).join('');
  });
  rf.addEventListener('submit', async e => {
    e.preventDefault();
    const err = rf.querySelector('.error');
    const pw = rf.new_password.value;
    if (pw.length < 8) { showError(err, 'Password must be at least 8 characters.'); return; }
    showError(err, '');
    if (!await confirmDialog({ title: 'Set a new password?', message: `${current.student.name} will be logged out and must use the new password.`, confirmText: 'Set password' })) return;
    try {
      await withLoading(rf.querySelector('button[type=submit]'), () => api(`/api/admin/students/${current.student.id}/password`, { method: 'POST', body: { new_password: pw } }));
      copyText(pw, 'Password set and copied. Share it with the student.');
      rf.reset();
    } catch (ex) { showError(err, ex.message); }
  });
}

async function grantAccess(e) {
  e.preventDefault();
  const f = e.target, err = document.getElementById('grantErr');
  const course = current.courses.find(c => c.id === Number(f.course_id.value));
  const days = Number(f.days.value);
  if (!days || days < 1) { showError(err, 'Enter the number of days.'); return; }
  showError(err, '');
  const ok = await confirmDialog({
    title: 'Give access?',
    message: `${current.student.name} gets "${course.title}" for ${days} days. Amount recorded: ${rupees(f.amount.value)}.`,
    confirmText: 'Give access',
  });
  if (!ok) return;
  try {
    await withLoading(f.querySelector('button[type=submit]'), () => api(`/api/admin/students/${current.student.id}/grant`, { method: 'POST', body: {
      course_id: course.id, days, start: f.start.value,
      amount: Number(f.amount.value || 0), note: f.note.value,
      claim_id: f.claim_id && f.claim_id.value ? Number(f.claim_id.value) : null,
    }}));
    showToast(`Access given to ${current.student.name}.`, 'success');
    afterChange();
  } catch (ex) {
    if (ex.status === 401) return location.replace('admin-login.html');
    showError(err, ex.message);
  }
}

async function revoke(enrollmentId) {
  if (!await confirmDialog({
    title: 'Revoke this access?', message: 'The student loses access for this period right away. The payment record is kept.',
    confirmText: 'Revoke', danger: true,
  })) return;
  await guarded(() => api(`/api/admin/enrollments/${enrollmentId}/revoke`, { method: 'POST' }));
  showToast('Access revoked.', 'success');
  afterChange();
}

// ---------- Payment notes ----------

// One payment note: who paid, every detail they sent, and Screenshot / Accept / Reject.
function noteCard(c) {
  const details = [
    ['Paid to', c.qr_slot ? esc(qrLabel(c.qr_slot)) : ''], ['Paid using', esc(c.method)], ['Account name', esc(c.payer_name)],
    ['Payment date', esc(c.paid_on)], ['UTR / transaction ID', c.reference ? `<span class="mono">${esc(c.reference)}</span>` : ''],
    ['Age', c.age ? String(c.age) : ''], ['Referred by', esc(referralText(c))],
  ].filter(([, v]) => v);
  return `<article class="card note-card">
    <div class="note-head">
      <div class="who"><span class="avatar-sm">${esc(initials(c.student_name))}</span><div style="min-width:0"><b>${esc(c.student_name)}</b>
        <span>${[c.student_code && esc(c.student_code), c.student_phone && `<a href="${esc(telHref(c.student_phone))}">${esc(c.student_phone)}</a>`, 'sent ' + timeAgo(c.created_at)].filter(Boolean).join(' · ')}</span></div></div>
      <div class="note-amount"><b>${c.amount != null ? rupees(c.amount) : '—'}</b><span>${esc(c.course_title)}</span></div>
    </div>
    <dl class="note-grid">${details.map(([k, v]) => `<div><dt>${k}</dt><dd>${v}</dd></div>`).join('')}</dl>
    <div class="note-actions">
      ${shotButton(c)}
      <span class="grow"></span>
      <button class="btn danger sm" data-dismiss="${c.id}">${icon('x')} Reject</button>
      <button class="btn success sm" data-open="${c.student_id}" data-claim="${c.id}">${icon('check')} Accept</button>
    </div>
  </article>`;
}

async function viewNotes() {
  setView(head('Money', 'Payment notes', 'Details students sent after paying. Match them with your bank statement, then verify.') + skeleton(240));
  const rows = await guarded(() => api('/api/admin/claims?status=pending'));
  setPendingCount(rows.length);
  setView(`
    ${head('Money', 'Payment notes', 'Details students sent after paying. Match them with your bank statement, then accept to give course access.')}
    ${rows.length ? `<div class="stack">${rows.map(noteCard).join('')}</div>`
      : emptyState('checkCircle', 'All caught up', 'No payment notes are waiting. New ones appear here as soon as a student sends them.')}
  `);
  const view = document.getElementById('view');
  view.querySelectorAll('[data-open]').forEach(b => b.addEventListener('click', () => openStudent(Number(b.dataset.open), Number(b.dataset.claim))));
  wireShots(view);
  view.querySelectorAll('[data-dismiss]').forEach(b => b.addEventListener('click', async () => {
    if (!await confirmDialog({ title: 'Reject this payment note?', message: 'Do this only if no matching payment arrived in your account. The student can send their details again.', confirmText: 'Reject', danger: true })) return;
    await guarded(() => api(`/api/admin/claims/${b.dataset.dismiss}/dismiss`, { method: 'POST' }));
    showToast('Payment note rejected.', 'success');
    viewNotes();
  }));
}

// ---------- Payments ----------

async function viewPayments() {
  setView(head('Money', 'Payments', 'Every access you have given, with the amount received.') + skeleton(300));
  const [list, o] = await guarded(() => Promise.all([api('/api/admin/payments'), api('/api/admin/overview')]));
  payments = list;
  const t = nowTs();
  setView(`
    ${head('Money', 'Payments', 'Every access you have given, with the amount received.',
      `<button class="btn outline sm" id="csvBtn">${icon('download')} Download CSV</button>`)}
    <div class="stats" style="margin-bottom:18px">
      <div class="stat-tile"><span class="stat-ico b">${icon('wallet')}</span><span><small>This month</small><b>${rupees(o.collected_this_month)}</b></span></div>
      <div class="stat-tile"><span class="stat-ico">${icon('rupee')}</span><span><small>Total collected</small><b>${rupees(o.total_collected)}</b></span></div>
      <div class="stat-tile"><span class="stat-ico g">${icon('list')}</span><span><small>Records</small><b>${list.length}</b></span></div>
    </div>
    ${list.length ? `<div class="card table-wrap"><table class="responsive">
      <thead><tr><th>Student</th><th>Course</th><th class="num">Amount</th><th>Access period</th><th>Notes</th><th>Status</th><th>Receipt</th></tr></thead>
      <tbody>${list.map(p => {
        const status = p.revoked_at ? badge('expired', 'Revoked') : p.end_at <= t ? badge('none', 'Ended') : p.start_at > t ? badge('scheduled') : badge('active');
        return `<tr class="click" data-id="${p.student_id}">
          <td class="primary-cell"><div class="who"><span class="avatar-sm">${esc(initials(p.student_name))}</span><div style="min-width:0"><b>${esc(p.student_name)}</b><span>${fmtDate(p.granted_at)} · ${esc(p.student_email)}</span></div></div></td>
          <td data-label="Course">${esc(p.course_title)}</td><td data-label="Amount" class="num strong">${rupees(p.amount_paid)}</td>
          <td data-label="Period" class="small">${fmtDate(p.start_at)} → ${fmtDate(p.end_at)}</td>
          <td data-label="Notes" class="small hide-sm">${esc(p.note) || '—'}</td><td data-label="Status">${status}</td>
          <td class="actions">${adminReceiptButton(p)}</td>
        </tr>`;
      }).join('')}</tbody></table></div>`
      : emptyState('wallet', 'No payments yet', 'When you give a student access, the payment is recorded here.')}
  `);
  document.getElementById('csvBtn').addEventListener('click', downloadCsv);
  document.querySelectorAll('#view tr.click').forEach(tr => tr.addEventListener('click', () => openStudent(Number(tr.dataset.id))));
  wireAdminReceipts(document.getElementById('view'));
}

function downloadCsv() {
  const iso = ts => new Date(ts * 1000).toISOString().slice(0, 10);
  const cell = v => `"${String(v ?? '').replace(/"/g, '""')}"`;
  const lines = [['Receipt no', 'Given on', 'Student ID', 'Student', 'Email', 'Course', 'Amount', 'Start', 'End', 'Notes', 'Revoked on']]
    .concat(payments.map(p => [p.receipt_no, iso(p.granted_at), p.student_code || '', p.student_name, p.student_email, p.course_title,
      p.amount_paid, iso(p.start_at), iso(p.end_at), p.note, p.revoked_at ? iso(p.revoked_at) : '']));
  const blob = new Blob([lines.map(l => l.map(cell).join(',')).join('\n')], { type: 'text/csv' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `gcation-payments-${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(a.href);
}

// ---------- Management team ----------

const signupUrl = () => new URL('management-login.html#register', location.href).href;

async function viewManagement() {
  const sub = 'People who refer students. They see courses and prices and share their Management ID (GCM…) as the Referral ID.';
  setView(head('People', 'Management', sub) + skeleton(300));
  const data = await guarded(() => api('/api/admin/managers'));
  setView(`
    ${head('People', 'Management', sub, `<button class="btn outline sm" data-copy-url>${icon('copy')} Copy registration link</button>`)}
    <div class="notice info" style="margin-bottom:16px">${icon('info')}<div>New members register once with the <b>management referral code</b>
      ${data.invite_code ? `(now <b class="mono">${esc(data.invite_code)}</b>)` : '(<b>not set</b>, so registration is closed)'}. After that they log in with just their email and password.
      Change the code in <a href="#/account" style="font-weight:700">Admin account</a>.</div></div>
    ${data.items.length ? `<div class="card table-wrap"><table class="responsive">
      <thead><tr><th>Member</th><th>Management ID</th><th>Phone</th><th class="num">Approved referrals</th><th class="num">Waiting</th><th>Status</th><th></th></tr></thead>
      <tbody>${data.items.map(m => `<tr class="${m.revoked_at ? 'revoked' : ''}">
        <td class="primary-cell"><div class="who"><span class="avatar-sm">${esc(initials(m.name))}</span><div style="min-width:0"><b>${esc(m.name)}</b><span>${esc(m.email)} · joined ${fmtDate(m.created_at)}</span></div></div></td>
        <td data-label="Management ID" class="mono strong">${esc(m.manager_code)}</td>
        <td data-label="Phone">${m.phone ? `<a href="${esc(telHref(m.phone))}" class="strong">${esc(m.phone)}</a>` : '<span class="faint">—</span>'}</td>
        <td data-label="Approved" class="num strong">${m.approved}</td><td data-label="Waiting" class="num">${m.pending}</td>
        <td data-label="Status">${m.revoked_at ? badge('expired', 'Revoked ' + fmtDate(m.revoked_at)) : badge('active')}</td>
        <td class="right actions">${m.revoked_at
          ? `<button class="btn success sm" data-restore="${m.id}">${icon('check')} Restore</button>`
          : `<button class="btn danger sm" data-revoke-m="${m.id}">${icon('x')} Revoke</button>`}</td></tr>`).join('')}</tbody></table></div>`
      : emptyState('users', 'No management members yet', 'Set the referral code in Admin account and share the registration link with your team.')}
  `);
  document.querySelector('[data-copy-url]').addEventListener('click', () => copyText(signupUrl(), 'Registration link copied'));
  const byId = id => data.items.find(m => m.id === Number(id));
  document.querySelectorAll('[data-revoke-m]').forEach(b => b.addEventListener('click', async () => {
    const m = byId(b.dataset.revokeM);
    if (!await confirmDialog({ title: `Revoke ${m.name}'s access?`, message: `${m.name} is logged out and cannot log in, and students can no longer use ${m.manager_code} as a Referral ID. Their past referrals are kept. You can restore access any time.`, confirmText: 'Revoke access', danger: true })) return;
    await guarded(() => api(`/api/admin/managers/${m.id}/revoke`, { method: 'POST' }));
    showToast(`${m.name}'s access revoked.`, 'success');
    viewManagement();
  }));
  document.querySelectorAll('[data-restore]').forEach(b => b.addEventListener('click', async () => {
    const m = byId(b.dataset.restore);
    if (!await confirmDialog({ title: `Restore ${m.name}'s access?`, message: `${m.name} can log in again with the same email and password, and ${m.manager_code} works as a Referral ID again.`, confirmText: 'Restore access' })) return;
    await guarded(() => api(`/api/admin/managers/${m.id}/restore`, { method: 'POST' }));
    showToast(`${m.name} can log in again.`, 'success');
    viewManagement();
  }));
}

// ---------- Updates ----------

const TEMPLATES = [
  ['message', 'Community link', {
    title: 'Join our student community',
    message: 'Join the official G-CATION group for class updates, practice material and doubts.\nPlease keep the group respectful.',
    link_label: 'Join community', link_url: 'https://chat.whatsapp.com/',
  }],
  ['video', 'Google Meet class', {
    title: `Live class: ${new Date().toLocaleDateString('en-IN', { weekday: 'long', day: 'numeric', month: 'short' })}, 7:00 PM`,
    message: 'Please join 5 minutes early. Keep your mic muted until you are asked to speak.',
    link_label: 'Join Google Meet', link_url: 'https://meet.google.com/',
  }],
  ['list', 'Instructions', { title: 'Important instructions', message: '1. \n2. \n3. ', link_label: '', link_url: '' }],
];

async function viewUpdates() {
  editingAnn = null;
  setView(`
    ${head('Content', 'Updates', 'Share Google Meet links, the community group or instructions with the management team, all paid students or the students of chosen courses.')}
    <div class="panel-grid">
      <form class="card pad-lg" id="annForm" novalidate>
        <h3 class="card-title" id="composerTitle"></h3>
        <div class="templates" id="templates"></div>
        <div class="field"><label for="aTitle">Title</label><input id="aTitle" name="title" maxlength="120" placeholder="e.g. Live class today at 7 PM" required></div>
        <div class="field"><label for="aMsg">Message <span class="faint">(optional)</span></label><textarea id="aMsg" name="message" maxlength="3000" placeholder="Instructions, timings, rules…"></textarea></div>
        <div class="row2">
          <div class="field"><label for="aUrl">Link <span class="faint">(optional)</span></label><input id="aUrl" name="link_url" maxlength="500" placeholder="https://meet.google.com/…"></div>
          <div class="field"><label for="aLabel">Button text</label><input id="aLabel" name="link_label" maxlength="60" placeholder="Join Google Meet"></div>
        </div>
        <div class="field"><span class="label" id="audLabel">Who should see this?</span>
          <div class="multi" id="audBox">
            <button type="button" class="multi-btn" id="audBtn" aria-haspopup="true" aria-expanded="false" aria-labelledby="audLabel audSummary"><span id="audSummary"></span>${icon('chevron')}</button>
            <div class="multi-list" id="audList" role="group" aria-labelledby="audLabel" hidden></div>
          </div><small>Tick everyone who should get it.</small></div>
        <div class="preview-box"><div class="tiny faint strong" style="margin-bottom:8px">PREVIEW · WHAT THEY SEE</div><div id="annPreview"></div></div>
        <div class="error" id="annErr"></div>
        <div class="row" style="margin-top:14px">
          <button class="btn primary" type="submit" id="annSubmit"></button>
          <button class="btn outline" type="button" id="annCancel" hidden>Cancel edit</button>
        </div>
      </form>
      <div>
        <div class="section-title"><h2>Posted updates</h2><span class="muted small" id="annCount"></span></div>
        <div id="annList">${skeleton(200)}</div>
      </div>
    </div>
  `);
  const f = annForm();
  const tpl = document.getElementById('templates');
  tpl.innerHTML = '<span class="tiny faint strong">QUICK START</span>' +
    TEMPLATES.map(([ic, label], i) => `<button type="button" data-t="${i}">${icon(ic)} ${label}</button>`).join('');
  tpl.querySelectorAll('[data-t]').forEach(b => b.addEventListener('click', () => {
    const v = TEMPLATES[b.dataset.t][2];
    f.title.value = v.title; f.message.value = v.message; f.link_url.value = v.link_url; f.link_label.value = v.link_label;
    renderAnnPreview();
    (v.link_url ? f.link_url : f.message).focus();
  }));
  f.addEventListener('input', renderAnnPreview);
  f.addEventListener('submit', submitAnnouncement);
  wireAudience();
  document.getElementById('annCancel').addEventListener('click', () => resetComposer());
  if (!courses.length) courses = await guarded(() => api('/api/admin/courses'));
  resetComposer();
  await loadAnnouncements();
}

function annForm() { return document.getElementById('annForm'); }

function resetComposer(item = null) {
  editingAnn = item;
  const f = annForm();
  f.title.value = item ? item.title : '';
  f.message.value = item ? item.message : '';
  f.link_url.value = item ? item.link_url : '';
  f.link_label.value = item ? item.link_label : '';
  audienceOptions(item ? audienceOf(item) : ['all']);
  document.getElementById('composerTitle').innerHTML = icon(item ? 'edit' : 'megaphone') + (item ? ' Edit update' : ' New update');
  document.getElementById('annSubmit').innerHTML = item ? icon('check') + ' Save changes' : icon('send') + ' Post update';
  document.getElementById('annCancel').hidden = !item;
  showError(document.getElementById('annErr'), '');
  renderAnnPreview();
}

// Audience choices as checkbox values: 'management', 'all' (every paid student) and 'c<course id>'.
function audienceOf(a) {
  return [a.to_management && 'management', a.to_all_students && 'all', ...(a.course_ids || []).map(id => 'c' + id)].filter(Boolean);
}

// Adds https:// to a link typed without it (meet.google.com/abc → https://meet.google.com/abc).
function fullLink(v) {
  v = v.trim();
  if (!v) return '';
  return 'https://' + v.replace(/^(https?:)?\/+/i, '');
}

function chosenAudience() {
  return [...document.querySelectorAll('#audList input:checked')].map(i => i.value);
}

// Fills the "Who should see this?" list. With no `checked`, keeps what is ticked now.
function audienceOptions(checked = chosenAudience()) {
  const list = document.getElementById('audList');
  if (!list) return;
  const opts = [['management', 'users', 'Management team', audience.management],
    ['all', 'graduation', 'All paid students', audience.all],
    ...courses.map(c => ['c' + c.id, 'book', `${c.title} students`, audience[c.id]])];
  list.innerHTML = opts.map(([v, ic, label, n]) => `<label class="multi-opt">
      <input type="checkbox" value="${v}" ${checked.includes(v) ? 'checked' : ''}>
      ${icon(ic)}<span>${esc(label)}</span><b>${n || 0}</b></label>`).join('');
  syncAudience();
}

// Course boxes are covered by "All paid students", so they are shown ticked and locked.
function syncAudience() {
  const all = document.querySelector('#audList input[value=all]')?.checked;
  document.querySelectorAll('#audList input[value^=c]').forEach(i => {
    if (all) i.dataset.was ??= i.checked ? '1' : '';
    else if (i.dataset.was !== undefined) { i.checked = i.dataset.was === '1'; delete i.dataset.was; }
    i.disabled = !!all;
    if (all) i.checked = true;
  });
  const summary = document.getElementById('audSummary');
  if (summary) summary.textContent = audienceText() || 'Choose who should see this';
  summary?.classList.toggle('faint', !audienceText());
}

// e.g. "Management team, Course A students".
function audienceText() {
  const picked = chosenAudience();
  const all = picked.includes('all');
  return [...document.querySelectorAll('#audList input:checked')]
    .filter(i => !(all && i.value.startsWith('c')))
    .map(i => i.closest('label').querySelector('span').textContent).join(', ');
}

function wireAudience() {
  const box = document.getElementById('audBox'), btn = document.getElementById('audBtn'), list = document.getElementById('audList');
  const open = v => { list.hidden = !v; btn.setAttribute('aria-expanded', v); box.classList.toggle('open', v); };
  btn.addEventListener('click', () => open(list.hidden));
  list.addEventListener('change', syncAudience);
  document.addEventListener('click', e => { if (!box.contains(e.target)) open(false); });
  box.addEventListener('keydown', e => { if (e.key === 'Escape' && !list.hidden) { e.stopPropagation(); open(false); btn.focus(); } });
}

function renderAnnPreview() {
  const f = annForm();
  const title = f.title.value.trim() || 'Your title appears here';
  const msg = f.message.value.trim();
  const url = f.link_url.value.trim();
  document.getElementById('annPreview').innerHTML = `<article class="card update" style="box-shadow:none">
    <div class="update-icon">${icon(url.includes('meet.google') ? 'video' : url.includes('whatsapp') ? 'message' : url ? 'link' : 'megaphone', 'lg')}</div>
    <div style="min-width:0"><h3>${esc(title)} <span class="badge b-new">NEW</span></h3><div class="meta">just now</div>
      ${msg ? `<p class="msg">${linkify(msg)}</p>` : ''}
      ${url ? `<span class="btn primary sm">${icon('external')} ${esc(f.link_label.value.trim() || 'Open link')}</span>` : ''}</div></article>`;
}

async function submitAnnouncement(e) {
  e.preventDefault();
  const f = annForm(), err = document.getElementById('annErr');
  const body = {
    title: f.title.value.trim(), message: f.message.value, link_url: fullLink(f.link_url.value),
    link_label: f.link_label.value,
  };
  f.link_url.value = body.link_url;
  const picked = chosenAudience();
  body.to_management = picked.includes('management');
  body.to_all_students = picked.includes('all');
  body.course_ids = body.to_all_students ? [] : picked.filter(v => v.startsWith('c')).map(v => Number(v.slice(1)));
  if (!body.title) { showError(err, 'Add a title.'); return; }
  if (!body.to_management && !body.to_all_students && !body.course_ids.length) { showError(err, 'Choose who should see this update.'); return; }
  if (body.link_url && !/^https:\/\/\S+\.\S+/.test(body.link_url)) { showError(err, 'The link must be a full address starting with https://'); return; }
  if (/^https:\/\/(chat\.whatsapp\.com|meet\.google\.com)\/?$/.test(body.link_url)) { showError(err, 'Paste the full group or meeting link, not just the website.'); return; }
  showError(err, '');
  if (!editingAnn && !await confirmDialog({
    title: 'Post this update?', message: `${audienceText()} will see it right away.`, confirmText: 'Post update',
  })) return;
  try {
    await withLoading(document.getElementById('annSubmit'), () => editingAnn
      ? api(`/api/admin/announcements/${editingAnn.id}`, { method: 'PUT', body })
      : api('/api/admin/announcements', { method: 'POST', body }));
    showToast(editingAnn ? 'Update saved.' : 'Update posted.', 'success');
    resetComposer();
    loadAnnouncements();
  } catch (ex) {
    if (ex.status === 401) return location.replace('admin-login.html');
    showError(err, ex.message);
  }
}

async function loadAnnouncements() {
  const data = await guarded(() => api('/api/admin/announcements'));
  audience = data.audience;
  audienceOptions();
  const list = document.getElementById('annList');
  if (!list) return;
  document.getElementById('annCount').textContent = data.items.length ? `${data.items.length} posted` : '';
  if (!data.items.length) {
    list.innerHTML = emptyState('megaphone', 'Nothing posted yet', 'Use a quick start template to share your community link or a Google Meet class.');
    return;
  }
  list.innerHTML = data.items.map(a => `<div class="card ann-item">
    <div class="row between" style="align-items:flex-start"><h4>${esc(a.title)}</h4>${badge('info', a.audience_label)}</div>
    <div class="tiny muted">${fmtDateTime(a.created_at)}${a.updated_at > a.created_at + 60 ? ' · edited' : ''}</div>
    ${a.message ? `<p class="msg">${linkify(a.message)}</p>` : ''}
    ${a.link_url ? `<a class="small strong" style="color:var(--purple);word-break:break-all" href="${esc(a.link_url)}" target="_blank" rel="noopener">${icon('link')} ${esc(a.link_url)}</a>` : ''}
    <div class="row" style="margin-top:12px;gap:6px">
      <button class="btn outline sm" data-edit="${a.id}">${icon('edit')} Edit</button>
      <button class="btn danger sm" data-del="${a.id}">${icon('trash')} Delete</button>
    </div></div>`).join('');
  list.querySelectorAll('[data-edit]').forEach(b => b.addEventListener('click', () => {
    resetComposer(data.items.find(a => a.id === Number(b.dataset.edit)));
    annForm().scrollIntoView({ behavior: 'smooth', block: 'start' });
  }));
  list.querySelectorAll('[data-del]').forEach(b => b.addEventListener('click', async () => {
    if (!await confirmDialog({ title: 'Delete this update?', message: 'Nobody will see it any more.', confirmText: 'Delete', danger: true })) return;
    await guarded(() => api(`/api/admin/announcements/${b.dataset.del}`, { method: 'DELETE' }));
    if (editingAnn && editingAnn.id === Number(b.dataset.del)) resetComposer();
    showToast('Update deleted.', 'success');
    loadAnnouncements();
  }));
}

// ---------- Payment QR ----------

async function viewQr() {
  setView(head('Settings', 'Payment QR', 'Students see the Primary QR first and can switch to the Secondary QR if it fails.') + skeleton(360));
  const [methods, site] = await guarded(() => Promise.all([api('/api/admin/payment-methods'), api('/api/admin/settings')]));
  setView(`
    ${head('Settings', 'Payment QR', 'Students see the Primary QR first and can switch to the Secondary QR if it fails.')}
    <div class="notice info" style="margin-bottom:16px">${icon('info')}<div>Upload your QR image, or just enter a UPI ID and the site makes a QR with the course amount filled in. If you upload an image, students see that image.</div></div>
    <form class="card pad-lg" id="supportForm" novalidate style="margin-bottom:16px">
      <h3 class="card-title">${icon('phone')} Support phone number</h3>
      <p class="card-sub">Students are asked to call or WhatsApp this number after paying. It is also printed on receipts.</p>
      <div class="row" style="align-items:flex-end">
        <div class="field" style="flex:1;min-width:220px;margin:0"><label for="supPhone">Phone number</label>
          <input id="supPhone" name="contact_phone" type="tel" maxlength="20" placeholder="+91 98765 43210" value="${esc(site.contact_phone)}"></div>
        <button class="btn primary" type="submit">${icon('check')} Save</button>
      </div>
      <div class="error"></div>
    </form>
    <div class="stack">${methods.map(qrCard).join('')}</div>
  `);
  document.querySelectorAll('form[data-slot]').forEach(wireQrForm);
  const sf = document.getElementById('supportForm');
  sf.addEventListener('submit', async e => {
    e.preventDefault();
    const err = sf.querySelector('.error');
    showError(err, '');
    try {
      await withLoading(sf.querySelector('button[type=submit]'), () => api('/api/admin/settings', { method: 'PUT', body: { contact_phone: sf.contact_phone.value } }));
      showToast('Support number saved. Students see it right away.', 'success');
    } catch (ex) {
      if (ex.status === 401) return location.replace('admin-login.html');
      showError(err, ex.message);
    }
  });
}

function qrCard(m) {
  const preview = m.has_image
    ? `<img src="${API_BASE}/api/payment-methods/${m.slot}/image?v=${m.updated_at}" alt="${esc(m.label)} QR image">`
    : m.upi_id
      ? `<img data-auth-src="/api/admin/payment-methods/${m.slot}/preview.svg?v=${m.updated_at}" alt="QR generated from ${esc(m.upi_id)}">`
      : `<div class="qr-placeholder">${icon('qr')}<div>No QR yet</div></div>`;
  const showing = m.has_image ? 'your uploaded image' : m.upi_id ? 'a QR made from the UPI ID, with the amount filled in' : 'nothing — this option is hidden';
  return `<form class="card pad-lg" data-slot="${m.slot}" novalidate>
    <div class="row between" style="margin-bottom:14px">
      <h3 class="card-title" style="margin:0">${icon('qr')} ${m.slot === 'primary' ? 'Primary QR' : 'Secondary QR'}
        ${m.slot === 'primary' ? badge('info', 'Shown first') : badge('none', 'Backup')}</h3>
      <label class="switch"><input type="checkbox" name="is_active" ${m.is_active ? 'checked' : ''}> Show to students</label>
    </div>
    <div class="qr-admin">
      <div><div class="qr-frame">${preview}</div><p class="tiny muted center" style="margin:8px 0 0">Students see ${showing}.</p></div>
      <div>
        <div class="row2">
          <div class="field"><label>Name shown to students</label><input name="label" maxlength="60" value="${esc(m.label)}" placeholder="${m.slot === 'primary' ? 'Primary QR' : 'Secondary QR'}"></div>
          <div class="field"><label>Payee name</label><input name="payee_name" maxlength="80" value="${esc(m.payee_name)}" placeholder="G-CATION"></div>
        </div>
        <div class="field"><label>UPI ID</label><input name="upi_id" maxlength="120" value="${esc(m.upi_id)}" placeholder="yourname@okaxis" autocomplete="off" spellcheck="false">
          <small>Shown with a Copy button, and used for "Pay with UPI app" on phones.</small></div>
        <div class="field"><span class="label">QR image <span class="faint">(optional)</span></span>
          <label class="dropzone">${icon('upload', 'lg')}<div class="strong" style="margin-top:4px">${m.has_image ? 'Replace QR image' : 'Upload QR image'}</div>
            <div class="tiny">PNG or JPG · drop a file here or click to choose</div>
            <input type="file" accept="image/png,image/jpeg,image/webp" hidden></label>
          ${m.has_image ? `<button type="button" class="btn ghost sm" data-remove style="margin-top:6px">${icon('trash')} Remove image</button>` : ''}
        </div>
        <div class="error"></div>
        <div class="row"><button class="btn primary" type="submit">${icon('check')} Save</button><span class="tiny muted">Updated ${fmtDateTime(m.updated_at)}</span></div>
      </div>
    </div>
  </form>`;
}

function wireQrForm(f) {
  const slot = f.dataset.slot, err = f.querySelector('.error');
  const replace = m => { f.outerHTML = qrCard(m); wireQrForm(document.querySelector(`form[data-slot=${slot}]`)); };
  f.addEventListener('submit', async e => {
    e.preventDefault();
    showError(err, '');
    try {
      const m = await withLoading(f.querySelector('button[type=submit]'), () => api(`/api/admin/payment-methods/${slot}`, { method: 'PUT', body: {
        label: f.label.value, upi_id: f.upi_id.value, payee_name: f.payee_name.value, is_active: f.is_active.checked,
      }}));
      showToast('QR settings saved. Students see the change right away.', 'success');
      replace(m);
    } catch (ex) { showError(err, ex.message); }
  });
  const drop = f.querySelector('.dropzone'), input = drop.querySelector('input');
  const upload = async file => {
    showError(err, '');
    if (!file.type.startsWith('image/')) { showError(err, 'Choose an image file (PNG or JPG).'); return; }
    try {
      const data = await prepareImage(file);
      const m = await api(`/api/admin/payment-methods/${slot}/image`, { method: 'PUT', body: { data } });
      showToast('QR image uploaded. Students see the new QR right away.', 'success');
      replace(m);
    } catch (ex) {
      if (ex.status === 401) return location.replace('admin-login.html');
      showError(err, ex.message);
    }
  };
  input.addEventListener('change', () => input.files[0] && upload(input.files[0]));
  drop.addEventListener('dragover', e => { e.preventDefault(); drop.classList.add('over'); });
  drop.addEventListener('dragleave', () => drop.classList.remove('over'));
  drop.addEventListener('drop', e => { e.preventDefault(); drop.classList.remove('over'); if (e.dataTransfer.files[0]) upload(e.dataTransfer.files[0]); });
  const remove = f.querySelector('[data-remove]');
  if (remove) remove.addEventListener('click', async () => {
    if (!await confirmDialog({ title: 'Remove QR image?', message: 'Students will see a QR made from the UPI ID instead (if one is set).', confirmText: 'Remove', danger: true })) return;
    const m = await guarded(() => api(`/api/admin/payment-methods/${slot}/image`, { method: 'DELETE' }));
    showToast('QR image removed.', 'success');
    replace(m);
  });
}

// ---------- Courses ----------

function thumbBlock(c = {}) {
  return `<div data-thumb-preview>${c.id ? courseThumb(c) : `<div class="course-thumb t0">${icon('graduation')}</div>`}</div>
    <div>
      <span class="label">Thumbnail <span class="faint">(optional)</span></span>
      <label class="dropzone" style="margin-top:6px">${icon('upload')} <span class="strong">${c.has_thumbnail ? 'Replace image' : 'Upload image'}</span>
        <div class="tiny">PNG or JPG · a wide picture works best · shown on the website and to students</div>
        <input type="file" accept="image/png,image/jpeg,image/webp" hidden data-thumb-input></label>
      ${c.has_thumbnail ? `<button type="button" class="btn ghost sm" style="margin-top:6px" data-thumb-remove>${icon('trash')} Remove image</button>` : ''}
    </div>`;
}

function courseFields(c = {}) {
  return `
    <div class="thumb-edit" data-thumb>${thumbBlock(c)}</div>
    <div class="row2">
      <div class="field"><label>Title</label><input name="title" maxlength="120" required value="${esc(c.title)}"></div>
      <div class="field"><label>Status</label><select name="is_active">
        <option value="1" ${c.is_active !== false ? 'selected' : ''}>Open for enrollment</option>
        <option value="0" ${c.is_active === false ? 'selected' : ''}>Hidden</option></select></div>
    </div>
    <div class="field"><label>Description</label><textarea name="description" maxlength="1000">${esc(c.description)}</textarea></div>
    <div class="field"><label>Highlights <span class="faint">(optional, one per line)</span></label>
      <textarea name="features" maxlength="1500" placeholder="Live online classes&#10;Weekly speaking practice&#10;Certificate on completion">${esc(c.features)}</textarea>
      <small>Shown as a checklist on the website and to students. Leave empty to show none.</small></div>
    <div class="row2">
      <div class="field"><label>Price (₹)</label><input name="price" type="number" min="0" step="0.01" required value="${c.price != null ? Number(c.price) : ''}"></div>
      <div class="field"><label>Default access period (days)</label><input name="default_days" type="number" min="1" max="3650" required value="${c.default_days ?? 365}"></div>
    </div>
    <div class="field"><label>Permanent classroom link <span class="faint">(optional)</span></label>
      <input name="classroom_link" maxlength="500" placeholder="https://meet.google.com/…" value="${esc(c.classroom_link)}">
      <small>Shows as an "Open classroom" button for paid students. For one-off links, use Updates.</small></div>
    <div class="error"></div>`;
}

function readCourse(f) {
  return {
    title: f.title.value, description: f.description.value, features: f.features.value, price: Number(f.price.value),
    default_days: Number(f.default_days.value), classroom_link: f.classroom_link.value, is_active: f.is_active.value === '1',
  };
}

async function viewCourses() {
  setView(head('Settings', 'Courses', 'Thumbnail, price, access period and classroom link for each course.') + skeleton(360));
  courses = await guarded(() => api('/api/admin/courses'));
  setView(`
    ${head('Settings', 'Courses', 'Thumbnail, price, access period and classroom link for each course.')}
    <div class="stack">
      ${courses.map(c => `<details class="fold card" style="padding:0" ${courses.length === 1 ? 'open' : ''}>
        <summary><span class="row" style="gap:10px">${c.has_thumbnail ? `<img src="${thumbUrl(c)}" alt="" style="width:34px;height:34px;border-radius:9px;object-fit:cover">` : icon('book')} ${esc(c.title)} <span class="muted small" style="font-weight:600">${rupees(c.price)} · ${c.default_days} days</span>
          ${c.is_active ? badge('active', 'Open') : badge('none', 'Hidden')}</span></summary>
        <div class="fold-body">
          ${c.is_active ? '' : `<div class="notice warn" style="margin-bottom:6px;align-items:center">${icon('eyeOff')}<div style="flex:1">This course is <b>hidden</b>. It is not on the website and new students cannot enroll. Students who already paid keep their access.</div>
            <button class="btn primary sm" type="button" data-visibility="${c.id}">${icon('eye')} Show on website</button></div>`}
          <form data-id="${c.id}" novalidate>${courseFields(c)}
          <div class="row between"><button class="btn primary" type="submit">${icon('check')} Save changes</button>
          <div class="row" style="gap:6px">
            ${c.is_active ? `<button class="btn outline sm" type="button" data-visibility="${c.id}">${icon('eyeOff')} Hide from website</button>` : ''}
            <button class="btn danger sm" type="button" data-delete="${c.id}">${icon('trash')} Delete course</button></div></div></form></div>
      </details>`).join('')}
      <details class="fold card" style="padding:0">
        <summary><span class="row" style="gap:10px;color:var(--purple)">${icon('plus')} Add a new course</span></summary>
        <div class="fold-body"><form id="newCourse" novalidate>${courseFields()}<button class="btn primary" type="submit">${icon('plus')} Add course</button></form></div>
      </details>
    </div>
  `);
  let newThumb = null; // image chosen for a course that does not exist yet
  const save = async (f, method, path, done) => {
    const err = f.querySelector('.error');
    showError(err, '');
    try {
      await withLoading(f.querySelector('button[type=submit]'), async () => {
        const res = await api(path, { method, body: readCourse(f) });
        if (method === 'POST' && newThumb) await api(`/api/admin/courses/${res.id}/thumbnail`, { method: 'PUT', body: { data: newThumb } });
      });
      showToast(done, 'success');
      viewCourses();
    } catch (ex) {
      if (ex.status === 401) return location.replace('admin-login.html');
      showError(err, ex.message);
    }
  };
  document.querySelectorAll('form[data-id]').forEach(f => f.addEventListener('submit', e => { e.preventDefault(); save(f, 'PUT', '/api/admin/courses/' + f.dataset.id, 'Course saved.'); }));
  const nf = document.getElementById('newCourse');
  nf.addEventListener('submit', e => { e.preventDefault(); save(nf, 'POST', '/api/admin/courses', 'Course added.'); });
  document.querySelectorAll('[data-delete]').forEach(b => b.addEventListener('click', () => deleteCourse(courses.find(c => c.id === Number(b.dataset.delete)))));
  document.querySelectorAll('[data-visibility]').forEach(b => b.addEventListener('click', () => setCourseVisible(courses.find(c => c.id === Number(b.dataset.visibility)))));
  document.querySelectorAll('form[data-id]').forEach(f => wireThumb(f, courses.find(c => c.id === Number(f.dataset.id))));
  wireThumb(nf, null, data => { newThumb = data; });
}

// Thumbnail picker. Existing courses upload right away (other unsaved edits in the form are kept);
// a new course keeps the image until the course is created.
function wireThumb(form, course, onPick) {
  const box = form.querySelector('[data-thumb]');
  const err = form.querySelector('.error');
  const rewire = () => {
    const input = box.querySelector('[data-thumb-input]'), drop = input.closest('.dropzone');
    input.addEventListener('change', () => input.files[0] && pick(input.files[0]));
    drop.addEventListener('dragover', e => { e.preventDefault(); drop.classList.add('over'); });
    drop.addEventListener('dragleave', () => drop.classList.remove('over'));
    drop.addEventListener('drop', e => { e.preventDefault(); drop.classList.remove('over'); if (e.dataTransfer.files[0]) pick(e.dataTransfer.files[0]); });
    const remove = box.querySelector('[data-thumb-remove]');
    if (remove) remove.addEventListener('click', async () => {
      if (!await confirmDialog({ title: 'Remove thumbnail?', message: 'The course goes back to its colour gradient.', confirmText: 'Remove', danger: true })) return;
      update(await guarded(() => api(`/api/admin/courses/${course.id}/thumbnail`, { method: 'DELETE' })));
      showToast('Thumbnail removed.', 'success');
    });
  };
  const update = c => {
    Object.assign(course, c);
    box.innerHTML = thumbBlock(course);
    rewire();
  };
  const pick = async file => {
    showError(err, '');
    if (!file.type.startsWith('image/')) { showError(err, 'Choose an image file (PNG or JPG).'); return; }
    try {
      const data = await prepareImage(file);
      if (!course) {
        onPick(data);
        box.querySelector('[data-thumb-preview]').innerHTML = `<div class="course-thumb"><img src="${data}" alt=""></div>`;
        return;
      }
      update(await api(`/api/admin/courses/${course.id}/thumbnail`, { method: 'PUT', body: { data } }));
      showToast('Thumbnail updated. Students see it right away.', 'success');
    } catch (ex) {
      if (ex.status === 401) return location.replace('admin-login.html');
      showError(err, ex.message);
    }
  };
  rewire();
}

async function deleteCourse(c) {
  if (!await confirmDialog({
    title: `Delete "${c.title}"?`,
    message: 'The course and its updates are removed for good. Students will no longer see it.',
    confirmText: 'Delete course', danger: true,
  })) return;
  try {
    await api(`/api/admin/courses/${c.id}`, { method: 'DELETE' });
    showToast('Course deleted.', 'success');
    viewCourses();
  } catch (ex) {
    if (ex.status === 401) return location.replace('admin-login.html');
    if (ex.status !== 409) { showToast(ex.message, 'error'); return; }
    // Courses with payment records are hidden instead, so payment history is kept.
    if (!await confirmDialog({ title: 'Hide this course instead?', message: ex.message + ' Hidden courses disappear from the website, but paid students keep their access.', confirmText: 'Hide course' })) return;
    await setCourseVisible(c, false, false); // already confirmed above
  }
}

// Shows or hides a course on the website, keeping its saved details. With no `show`, it flips the current state.
async function setCourseVisible(c, show = !c.is_active, ask = true) {
  if (ask && !show && c.is_active && !await confirmDialog({
    title: `Hide "${c.title}"?`,
    message: 'It disappears from the website and new students cannot enroll. Students who already paid keep their access. You can show it again any time.',
    confirmText: 'Hide course',
  })) return;
  await guarded(() => api(`/api/admin/courses/${c.id}`, { method: 'PUT', body: {
    title: c.title, description: c.description, features: c.features, price: Number(c.price), default_days: c.default_days,
    classroom_link: c.classroom_link, is_active: show,
  }}));
  showToast(show ? 'Course is visible on the website again.' : 'Course hidden from the website.', 'success');
  viewCourses();
}

// ---------- Admin account ----------

async function viewAccount() {
  setView(head('Settings', 'Admin account', 'Change the username and password you use to log in here.') + skeleton(320));
  const [acc, team] = await guarded(() => Promise.all([api('/api/admin/account'), api('/api/admin/managers')]));
  setView(`
    ${head('Settings', 'Admin account', 'Change the username and password you use to log in here.')}
    <div class="grid-2" style="align-items:start">
      <form class="card pad-lg" id="accountForm" novalidate>
        <h3 class="card-title">${icon('key')} Login details</h3>
        <p class="card-sub">Other devices are logged out after a change. This device stays logged in.</p>
        <div class="field"><label for="accUser">Username</label><input id="accUser" name="username" maxlength="60" autocomplete="username" value="${esc(acc.username)}" required></div>
        <div class="field"><label for="accNew">New password <span class="faint">(leave empty to keep the current one)</span></label><input id="accNew" name="new_password" type="password" autocomplete="new-password"></div>
        <div class="field"><label for="accConfirm">Confirm new password</label><input id="accConfirm" name="confirm" type="password" autocomplete="new-password"></div>
        <hr class="divider">
        <div class="field"><label for="accCur">Current password</label><input id="accCur" name="current_password" type="password" autocomplete="current-password" required><small>Needed to save any change.</small></div>
        <div class="error"></div>
        <button class="btn primary" type="submit">${icon('check')} Save login details</button>
      </form>
      <div class="stack">
        <form class="card pad-lg" id="inviteForm" novalidate>
          <h3 class="card-title">${icon('shield')} Management referral code</h3>
          <p class="card-sub">New management members enter this once, when they register. People who already registered just log in with their email and password, even after you change it. Leave it empty to close registration.</p>
          <div class="row" style="align-items:flex-end">
            <div class="field" style="flex:1;min-width:180px;margin:0"><label for="invCode">Referral code</label>
              <input id="invCode" name="invite_code" maxlength="60" autocomplete="off" spellcheck="false" value="${esc(team.invite_code)}" placeholder="e.g. GCTEAM2026"></div>
            <button class="btn primary" type="submit">${icon('check')} Save</button>
          </div>
          <div class="error"></div>
          <button type="button" class="btn ghost sm" data-copy-url style="margin-top:8px">${icon('copy')} Copy registration link</button>
        </form>
        <div class="card pad-lg">
          <div class="row" style="gap:14px"><span class="avatar-sm avatar-lg">${esc(initials(acc.username))}</span>
            <div><div class="tiny faint strong">SIGNED IN AS</div><h2 style="margin:2px 0 0;font-size:1.3rem">${esc(acc.username)}</h2>
              <div class="muted small">Last changed ${fmtDateTime(acc.updated_at)}</div></div></div>
        </div>
        <div class="notice info">${icon('info')}<div><b>Forgot the admin password?</b> On the server, run
          <span class="mono">.venv/bin/python -m app.reset_admin</span> in the backend folder. It resets the login to
          ADMIN_USERNAME and ADMIN_PASSWORD from <span class="mono">backend/.env</span>.</div></div>
      </div>
    </div>
  `);
  setupPasswordToggles();
  document.querySelector('[data-copy-url]').addEventListener('click', () => copyText(signupUrl(), 'Registration link copied'));
  const inv = document.getElementById('inviteForm');
  inv.addEventListener('submit', async e => {
    e.preventDefault();
    const err = inv.querySelector('.error');
    showError(err, '');
    try {
      await withLoading(inv.querySelector('button[type=submit]'), () => api('/api/admin/managers/invite-code', { method: 'PUT', body: { invite_code: inv.invite_code.value } }));
      showToast(inv.invite_code.value.trim() ? 'Referral code saved.' : 'Management registration closed.', 'success');
    } catch (ex) {
      if (ex.status === 401) return location.replace('admin-login.html');
      showError(err, ex.message);
    }
  });
  const f = document.getElementById('accountForm');
  f.addEventListener('submit', async e => {
    e.preventDefault();
    const err = f.querySelector('.error');
    const username = f.username.value.trim(), pw = f.new_password.value;
    const problem = username.length < 3 || /\s/.test(username) ? 'Username needs at least 3 characters and no spaces.'
      : pw && pw.length < 8 ? 'New password must be at least 8 characters.'
      : pw !== f.confirm.value ? 'The new passwords do not match.'
      : !f.current_password.value ? 'Enter your current password to save.' : '';
    showError(err, problem);
    if (problem) return;
    try {
      const res = await withLoading(f.querySelector('button[type=submit]'), () => api('/api/admin/account', { method: 'PUT', body: {
        username, new_password: pw, current_password: f.current_password.value,
      }}));
      showToast(pw ? 'Username and password updated.' : 'Username updated.', 'success');
      viewAccount();
    } catch (ex) {
      if (ex.status === 401) return location.replace('admin-login.html');
      showError(err, ex.message);
    }
  });
}

// ---------- Boot ----------

document.querySelectorAll('[data-icon]').forEach(el => el.insertAdjacentHTML('afterbegin', icon(el.dataset.icon)));
document.getElementById('menuBtn').innerHTML = icon('menu');
document.getElementById('viewSite').innerHTML = icon('external') + ' View website';
document.getElementById('logoutBtn').innerHTML = icon('logout') + ' Log out';
document.getElementById('logoutBtn').addEventListener('click', async () => {
  await api('/api/admin/logout', { method: 'POST' }).catch(() => {});
  setToken('admin', '');
  location.href = 'admin-login.html';
});
setupSidebarToggle();
window.addEventListener('hashchange', route);
// Confirms the admin is logged in (redirects to login if not), then shows the page.
guarded(() => api('/api/admin/overview')).then(o => { setPendingCount(o.pending_claims); route(); hideSplash(); });
// Keep the "Payment notes" count fresh while the dashboard stays open.
setInterval(() => { if (!document.hidden) refreshCounts(); }, 60000);
