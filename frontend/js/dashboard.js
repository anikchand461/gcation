// Student app: Home, Courses, Checkout (#/enroll/:id), Updates and Profile.

let me = null;               // /api/me response
let updates = null;          // /api/me/announcements response
let knownUpdateIds = null;   // to spot new updates while the page is open
const enrollStep = {};       // course id -> 1 (pay) | 2 (details) | 3 (call)
const chosenSlot = {};       // course id -> selected QR slot
const resend = {};           // course id -> true when re-opening checkout after sending details
let seenMarked = false;

const VIEWS = { home: viewHome, courses: viewCourses, enroll: viewEnroll, updates: viewUpdates, profile: viewProfile };
const active = () => me.courses.filter(c => c.access.status === 'active');
const hasActive = () => active().length > 0;
const courseById = id => me.courses.find(c => c.id === Number(id));

// ---------- Data ----------

async function load() {
  try {
    me = await api('/api/me');
  } catch (e) {
    if (e.status === 401) { location.replace('login.html'); return false; }
    showToast(e.message, 'error');
    return false;
  }
  if (hasActive()) {
    updates = await api('/api/me/announcements').catch(() => updates);
    if (updates && knownUpdateIds === null) knownUpdateIds = new Set(updates.items.map(i => i.id));
  } else {
    updates = null;
  }
  renderChrome();
  return true;
}

function renderChrome() {
  const s = me.student;
  for (const id of ['sideAvatar', 'topAvatar']) document.getElementById(id).textContent = initials(s.name);
  document.getElementById('sideName').textContent = s.name;
  document.getElementById('sideEmail').textContent = s.email;
  const n = me.unread_announcements;
  const navUnread = document.getElementById('navUnread');
  navUnread.textContent = n; navUnread.hidden = !n;
  document.getElementById('bellBtn').innerHTML = icon('bell') + (n ? `<span class="dot-count">${n > 9 ? '9+' : n}</span>` : '');
  const tab = document.querySelector('.tabbar [data-route=updates]');
  tab.querySelector('.dot')?.remove();
  if (n) tab.insertAdjacentHTML('beforeend', '<span class="dot"></span>');
}

// ---------- Router ----------

async function route() {
  let { name, params } = currentRoute();
  if (!VIEWS[name]) { name = 'home'; history.replaceState(null, '', '#/home'); }
  markNav(name === 'enroll' ? 'courses' : name);
  if (!me) {
    if (await load()) VIEWS[name](...params);
    hideSplash();
    return;
  }
  // Show cached data instantly, then refresh in the background so newly granted access,
  // new updates or a logged-out session are picked up on every page switch.
  VIEWS[name](...params);
  const before = JSON.stringify([me, updates]);
  if (!(await load()) || JSON.stringify([me, updates]) === before) return;
  const now = currentRoute();
  const typing = now.name === 'profile' || (now.name === 'enroll' && enrollStep[now.params[0]] === 2);
  if (now.name === name && !typing) VIEWS[name](...params);
}

// Re-fetch, then re-render the current view (after an action).
async function refresh() {
  if (await load()) { const { name, params } = currentRoute(); (VIEWS[name] || viewHome)(...params); }
}

// ---------- Shared pieces ----------

function durationLabel(days) {
  if (days % 365 === 0) return (days / 365) + (days === 365 ? ' year' : ' years');
  if (days % 30 === 0) return (days / 30) + (days === 30 ? ' month' : ' months');
  return days + ' days';
}

function statusBadge(c) {
  const a = c.access;
  if (a.status === 'active') return a.days_left <= 7 ? badge('pending', `${a.days_left} days left`) : badge('active');
  if (a.status === 'scheduled') return badge('scheduled');
  if (c.pending_claim) return badge('pending', 'Verifying payment');
  if (a.status === 'expired') return badge('expired');
  return '';
}

function classroomButton(c, cls = 'primary') {
  return c.classroom_link
    ? `<a class="btn ${cls} sm" href="${esc(c.classroom_link)}" target="_blank" rel="noopener">${icon('video')} Open classroom</a>`
    : `<a class="btn ${cls === 'primary' ? 'soft' : cls} sm" href="#/updates">${icon('megaphone')} Class links</a>`;
}

function learnCard(c) {
  const a = c.access;
  const total = Math.max(1, a.until - a.since);
  const left = Math.max(0, a.until - Date.now() / 1000);
  const pct = Math.round((left / total) * 100);
  return `<article class="card learn-card">
    <div class="ring" style="--p:${pct}" aria-label="${a.days_left} days left"><span><div><b>${a.days_left}</b><small>DAYS</small></div></span></div>
    <div class="info">
      <h3>${esc(c.title)}</h3>
      <div class="muted small">Valid until ${fmtDate(a.until)}</div>
      <div class="row" style="margin-top:10px">${classroomButton(c)}${latestReceipt(c.id) ? receiptButton(latestReceipt(c.id), 'ghost') : ''}${a.days_left <= 14 ? `<a class="btn outline sm" href="#/enroll/${c.id}">Renew</a>` : ''}</div>
    </div>
  </article>`;
}

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
      <div class="meta">${timeAgo(u.created_at)} · ${u.course_title ? esc(u.course_title) : 'All students'}${u.updated_at > u.created_at + 60 ? ' · edited' : ''}</div>
      ${u.message ? `<p class="msg">${linkify(u.message)}</p>` : ''}
      ${u.link_url ? `<a class="btn primary sm" href="${esc(u.link_url)}" target="_blank" rel="noopener">${icon('external')} ${esc(u.link_label || 'Open link')}</a>` : ''}
    </div>
  </article>`;
}

function wireCopy(root = document) {
  root.querySelectorAll('[data-copy]').forEach(b => b.addEventListener('click', () => copyText(b.dataset.copy, b.dataset.copyLabel || 'Copied')));
}

// ---------- Home ----------

function viewHome() {
  const s = me.student;
  const act = me.courses.filter(c => ['active', 'scheduled'].includes(c.access.status));
  const attention = [];
  for (const c of me.courses) {
    if (c.pending_claim && c.access.status !== 'active') {
      attention.push(['warn', 'hourglass', `Payment for ${c.title} is being verified`, 'Give us a quick call if you have not already.', `#/enroll/${c.id}`, 'View status']);
    }
    if (c.access.status === 'active' && c.access.days_left <= 14) {
      attention.push(['info', 'clock', `${c.title} ends in ${c.access.days_left} days`, `Renew to keep learning after ${fmtDate(c.access.until)}.`, `#/enroll/${c.id}`, 'Renew']);
    }
    if (c.access.status === 'expired' && !c.pending_claim) {
      attention.push(['info', 'alert', `${c.title} has ended`, `Your access ended on ${fmtDate(c.access.until)}.`, `#/enroll/${c.id}`, 'Renew']);
    }
  }
  if (me.unread_announcements) {
    attention.unshift(['ok', 'megaphone', `${me.unread_announcements} new update${me.unread_announcements > 1 ? 's' : ''} from G-CATION`, 'Class links, community and instructions.', '#/updates', 'Open']);
  }
  const latest = updates ? updates.items.slice(0, 2) : [];

  setView(`
    <section class="hero-card on-hero">
      <span class="kicker">${greeting()}</span>
      <h2>Hello, ${esc(s.name.split(' ')[0])}</h2>
      <p>${act.length ? `You have ${act.length} active course${act.length > 1 ? 's' : ''}. Keep going!` : 'Pick a course to start your learning journey.'}</p>
      <div class="row" style="margin-top:16px">
        <span class="chip">Student ID <b class="mono">${s.id}</b> <button type="button" data-copy="${s.id}" data-copy-label="Student ID copied">Copy</button></span>
        ${act.length ? '' : `<a class="btn white sm" href="#/courses">${icon('graduation')} Browse courses</a>`}
      </div>
    </section>

    ${attention.length ? `<div class="stack" style="margin-top:18px">${attention.map(([tone, ic, title, text, href, cta]) => `
      <a class="attention ${tone}" href="${href}"><span class="ico">${icon(ic)}</span>
        <span class="txt"><b>${esc(title)}</b><span class="muted">${esc(text)}</span></span>
        <span class="btn outline sm">${cta} ${icon('arrowRight')}</span></a>`).join('')}</div>` : ''}

    <div style="margin-top:26px">
      <div class="section-title"><h2>My learning</h2><a href="#/courses">All courses →</a></div>
      ${act.length ? `<div class="grid-2">${act.map(c => c.access.status === 'active' ? learnCard(c)
        : `<article class="card learn-card"><div class="info"><h3>${esc(c.title)}</h3><div class="muted small">Starts later · runs until ${fmtDate(c.access.until)}</div></div>${badge('scheduled')}</article>`).join('')}</div>`
        : emptyState('graduation', 'No active course yet', 'Choose a course, pay with UPI and we will unlock it after a quick verification.',
            `<a class="btn primary" href="#/courses">${icon('arrowRight')} Explore courses</a>`)}
    </div>

    ${latest.length ? `<div style="margin-top:26px">
      <div class="section-title"><h2>Latest updates</h2><a href="#/updates">See all →</a></div>
      ${latest.map(u => updateCard(u, updates.seen_at)).join('')}
    </div>` : ''}
  `);
  wireCopy();
  wireReceipts();
}

// ---------- Courses ----------

function courseCard(c) {
  const a = c.access;
  let meta, actions;
  if (a.status === 'active') {
    meta = `Valid until <b>${fmtDate(a.until)}</b>`;
    const r = latestReceipt(c.id);
    actions = classroomButton(c) + (r ? receiptButton(r, 'ghost') : '') + `<a class="btn ghost sm" href="#/enroll/${c.id}">Renew</a>`;
  } else if (a.status === 'scheduled') {
    meta = `Runs until <b>${fmtDate(a.until)}</b>`;
    actions = '';
  } else if (c.pending_claim) {
    meta = `<span class="price-tag">${rupees(c.price)}</span> <small class="muted">/ ${durationLabel(c.default_days)}</small>`;
    actions = `<a class="btn outline sm" href="#/enroll/${c.id}">${icon('hourglass')} View status</a>`;
  } else {
    meta = `<span class="price-tag">${rupees(c.price)}</span> <small class="muted">/ ${durationLabel(c.default_days)}</small>`;
    actions = `<a class="btn primary sm" href="#/enroll/${c.id}">${a.status === 'expired' ? 'Renew' : 'Enroll'} ${icon('arrowRight')}</a>`;
  }
  return `<article class="card course-card">
    ${courseThumb(c)}
    <div class="row between" style="align-items:flex-start"><h3>${esc(c.title)}</h3>${statusBadge(c)}</div>
    ${c.description ? `<p class="desc">${esc(c.description)}</p>` : ''}
    ${a.status === 'none' && c.features.length ? `<ul class="mini-features">${c.features.slice(0, 3).map(f => `<li>${icon('check')} ${esc(f)}</li>`).join('')}</ul>` : ''}
    <div class="foot"><div class="small">${meta}</div><div class="row" style="gap:6px">${actions}</div></div>
  </article>`;
}

function latestReceipt(courseId) {
  return me.receipts.find(r => r.course_id === courseId && !r.revoked_at);
}

function receiptButton(r, style = 'outline', label = 'Receipt') {
  return `<button type="button" class="btn ${style} sm" data-receipt="${r.id}" data-no="${esc(r.receipt_no)}">${icon('download')} ${label}</button>`;
}

function wireReceipts(root = document) {
  root.querySelectorAll('[data-receipt]').forEach(b => b.addEventListener('click', () =>
    withLoading(b, () => downloadFile(`/api/me/receipts/${b.dataset.receipt}.pdf`, `G-CATION-receipt-${b.dataset.no}.pdf`))
      .catch(e => showToast(e.message, 'error'))));
}

function viewCourses() {
  const mine = me.courses.filter(c => c.access.status !== 'none');
  const open = me.courses.filter(c => c.access.status === 'none' && c.is_open);
  setView(`
    <div class="view-head"><div><span class="kicker">Courses</span><h1>Your learning</h1><p>Enrolled courses and courses you can join.</p></div></div>
    ${mine.length ? `<div class="section-title"><h2>Enrolled</h2></div><div class="grid-3">${mine.map(courseCard).join('')}</div>` : ''}
    <div style="margin-top:${mine.length ? 28 : 0}px">
      <div class="section-title"><h2>${mine.length ? 'More courses' : 'Available courses'}</h2></div>
      ${open.length ? `<div class="grid-3">${open.map(courseCard).join('')}</div>`
        : emptyState('sparkles', mine.length ? 'You are enrolled in everything' : 'New batches coming soon', mine.length ? 'New courses will show up here.' : 'Check back shortly for upcoming courses.')}
    </div>
  `);
  wireReceipts();
}

// ---------- Checkout ----------

function viewEnroll(id) {
  const c = courseById(id);
  if (!c) { go('courses'); return; }
  const showStatus = c.pending_claim && !resend[c.id] && c.access.status !== 'active';
  const step = enrollStep[c.id] || 1;

  setView(`
    <div class="checkout">
      <a class="back-link" href="#/courses">${icon('chevron')} Back to courses</a>
      <div class="card summary">
        ${courseThumb(c)}
        <div style="flex:1;min-width:0"><h2>${esc(c.title)}</h2><div class="muted small">${durationLabel(c.default_days)} access</div></div>
        <div class="right"><div class="price-tag">${rupees(c.price)}</div>${statusBadge(c)}</div>
      </div>
      ${c.features.length ? `<ul class="mini-features card" style="padding:14px 18px;margin-top:10px">${c.features.map(f => `<li>${icon('check')} ${esc(f)}</li>`).join('')}</ul>` : ''}
      ${c.access.status === 'active' ? `<div class="notice info" style="margin-top:14px">${icon('info')}<div>You already have access until <b>${fmtDate(c.access.until)}</b>. Paying again adds ${durationLabel(c.default_days)} after that date.</div></div>` : ''}
      ${showStatus ? statusPanel(c) : `${wizardHead(step)}<div id="stepBody">${[null, stepPay, stepDetails, stepCall][step](c)}</div>`}
    </div>
  `);
  wireCheckout(c);
}

function wizardHead(step) {
  const labels = ['Pay', 'Share details', 'Confirm'];
  return `<div class="wizard">${labels.map((l, i) => {
    const n = i + 1, cls = n < step ? 'done' : n === step ? 'now' : '';
    return `<div class="w-step ${cls}"><i>${n < step ? '✓' : n}</i><span>${l}</span></div>`;
  }).join('')}</div>`;
}

function stepPay(c) {
  const methods = me.payment.methods;
  if (!methods.length) {
    return `<div class="card"><div class="notice warn">${icon('alert')}<div>Payment details are being updated. Please call us at <b>${esc(me.payment.contact_phone || 'the G-CATION office')}</b> to pay.</div></div></div>`;
  }
  const slot = methods.some(m => m.slot === chosenSlot[c.id]) ? chosenSlot[c.id] : methods[0].slot;
  chosenSlot[c.id] = slot;
  const m = methods.find(x => x.slot === slot);
  // Uploaded QR images are public; the generated QR needs the student's login.
  const imgAttr = m.has_image
    ? `src="${API_BASE}/api/payment-methods/${m.slot}/image?v=${m.updated_at}"`
    : `data-auth-src="/api/me/qr/${c.id}.svg?slot=${m.slot}&v=${m.updated_at}"`;
  const upiLink = m.upi_id
    ? `upi://pay?pa=${encodeURIComponent(m.upi_id).replace('%40', '@')}&pn=${encodeURIComponent(m.payee_name || 'G-CATION')}`
      + `&am=${Number(c.price).toFixed(2)}&cu=INR&tn=${encodeURIComponent('GCATION ID ' + me.student.id)}`
    : '';
  return `<div class="card pad-lg">
    <div class="pay-card">
      <div class="qr-col stack">
        ${methods.length > 1 ? `<div class="seg" role="tablist">${methods.map(x =>
          `<button type="button" role="tab" data-slot="${x.slot}" class="${x.slot === slot ? 'on' : ''}" aria-selected="${x.slot === slot}">${esc(x.label || (x.slot === 'primary' ? 'Primary QR' : 'Secondary QR'))}</button>`).join('')}</div>` : ''}
        <div class="qr-frame"><span class="corner c1"></span><span class="corner c2"></span><span class="corner c3"></span><span class="corner c4"></span>
          <img ${imgAttr} alt="UPI QR code: ${esc(m.label)}" width="300" height="300"></div>
        ${methods.length > 1 ? `<p class="hint center" style="margin:0">QR not working? Try the other one above.</p>` : ''}
      </div>
      <div class="stack">
        <div><div class="muted small">Amount to pay</div><div class="amount">${rupees(c.price)}</div></div>
        ${m.upi_id ? `<div class="upi-row"><div><small>UPI ID${m.payee_name ? ' · ' + esc(m.payee_name) : ''}</small>${esc(m.upi_id)}</div>
          <button type="button" class="btn outline sm" data-copy="${esc(m.upi_id)}" data-copy-label="UPI ID copied">${icon('copy')} Copy</button></div>` : ''}
        ${upiLink ? `<a class="btn primary block mobile-only" href="${esc(upiLink)}">${icon('smartphone')} Pay with UPI app</a>` : ''}
        <ol class="steps-list">
          <li>Scan the QR with GPay, PhonePe, Paytm or BHIM.</li>
          <li>Pay exactly ${rupees(c.price)}.</li>
          <li>Keep the payment screenshot or UTR number.</li>
        </ol>
        <button class="btn primary lg block" data-next="2">I have paid ${icon('arrowRight')}</button>
      </div>
    </div>
  </div>`;
}

function stepDetails(c) {
  const methods = me.payment.methods;
  const slot = chosenSlot[c.id] || (methods[0] && methods[0].slot) || '';
  return `<form class="card pad-lg" id="claimForm" novalidate>
    <h3 class="card-title">${icon('send')} Share your payment details</h3>
    <p class="card-sub">This helps us find your payment in seconds. Everything is optional except where you paid.</p>
    ${methods.length > 1 ? `<div class="field"><span class="label">Which QR did you pay to?</span>
      <div class="radio-cards">${methods.map(m => `<label><input type="radio" name="qr_slot" value="${m.slot}" ${m.slot === slot ? 'checked' : ''}> ${esc(m.label)}</label>`).join('')}</div></div>`
      : `<input type="hidden" name="qr_slot" value="${slot}">`}
    <div class="row2">
      <div class="field"><label for="cPayer">Name on bank / UPI account</label><input id="cPayer" name="payer_name" maxlength="80" value="${esc(me.student.name)}"></div>
      <div class="field"><label for="cMethod">Paid using</label><input id="cMethod" name="method" maxlength="60" placeholder="e.g. PhonePe, SBI"></div>
    </div>
    <div class="row2">
      <div class="field"><label for="cAmount">Amount paid (₹)</label><input id="cAmount" name="amount" type="number" min="0" step="0.01" value="${Number(c.price)}"></div>
      <div class="field"><label for="cDate">Date paid</label><input id="cDate" name="paid_on" type="date" value="${new Date().toISOString().slice(0, 10)}"></div>
    </div>
    <div class="field"><label for="cRef">UTR / transaction ID <span class="faint">(optional)</span></label><input id="cRef" name="reference" maxlength="80" placeholder="12-digit number from your UPI app" inputmode="numeric"></div>
    <div class="error"></div>
    <div class="row between" style="margin-top:8px">
      <button type="button" class="btn ghost" data-next="1">${icon('chevron')} Back</button>
      <button type="submit" class="btn primary lg">${icon('send')} Send details</button>
    </div>
  </form>`;
}

function stepCall(c) {
  return `<div class="card pad-lg">
    <div class="center" style="margin-bottom:18px">
      <div class="empty-state" style="padding:4px 0 0"><div class="ico" style="background:var(--green-bg);color:var(--green)">${icon('checkCircle', 'xl')}</div></div>
      <h3 style="margin:0 0 4px;font-size:1.3rem">Almost done!</h3>
      <p class="muted" style="margin:0">Call us and tell us your Student ID <b class="mono">${me.student.id}</b>. We will unlock your course right after.</p>
    </div>
    ${callBox(c)}
    <div class="row between" style="margin-top:18px">
      <span class="tiny muted">Your course appears on Home as soon as it is verified.</span>
      <a class="btn outline" href="#/home">Go to home</a>
    </div>
  </div>`;
}

function callBox(c) {
  const phone = me.payment.contact_phone;
  if (!phone) return `<div class="notice info">${icon('info')}<div>The G-CATION team will verify your payment shortly.</div></div>`;
  const msg = `Hi G-CATION, I have paid ${rupees(c.price)} for "${c.title}". My Student ID is ${me.student.id} (${me.student.email}).`;
  return `<div class="call-box">
    <div><div class="tiny muted strong">G-CATION SUPPORT</div><div class="phone">${esc(phone)}</div></div>
    <div class="row" style="gap:8px">
      <a class="btn primary" href="${esc(telHref(phone))}">${icon('phone')} Call</a>
      <a class="btn outline" href="${esc(waHref(phone, msg))}" target="_blank" rel="noopener">${icon('message')} WhatsApp</a>
    </div>
  </div>`;
}

function statusPanel(c) {
  const p = c.pending_claim;
  const method = me.payment.methods.find(m => m.slot === p.qr_slot);
  return `<div class="grid-2" style="margin-top:18px;align-items:start">
    <div class="card pad-lg">
      <h3 class="card-title">${icon('hourglass')} Verification in progress</h3>
      <p class="card-sub">The admin checks the bank account and unlocks your course.</p>
      <ol class="timeline">
        <li class="done"><span class="t-dot">${icon('check')}</span><div><b>Account created</b><span>Student ID ${me.student.id}</span></div></li>
        <li class="done"><span class="t-dot">${icon('check')}</span><div><b>Payment details sent</b><span>${fmtDateTime(p.created_at)}</span></div></li>
        <li class="now"><span class="t-dot">${icon('hourglass')}</span><div><b>Admin verification</b><span>Usually within a few hours</span></div></li>
        <li><span class="t-dot">${icon('graduation')}</span><div><b>Course unlocks</b><span>Class links appear in Updates</span></div></li>
      </ol>
    </div>
    <div class="stack">
      <div class="card">
        <h3 class="card-title">${icon('list')} What you sent</h3>
        <dl class="sent-list" style="margin-top:12px">
          ${method ? `<dt>Paid to</dt><dd>${esc(method.label)}</dd>` : ''}
          ${p.payer_name ? `<dt>Payer</dt><dd>${esc(p.payer_name)}</dd>` : ''}
          ${p.method ? `<dt>Using</dt><dd>${esc(p.method)}</dd>` : ''}
          ${p.amount != null ? `<dt>Amount</dt><dd>${rupees(p.amount)}</dd>` : ''}
          ${p.paid_on ? `<dt>Date</dt><dd>${esc(p.paid_on)}</dd>` : ''}
          ${p.reference ? `<dt>UTR</dt><dd class="mono">${esc(p.reference)}</dd>` : ''}
        </dl>
        <button type="button" class="btn ghost sm" style="margin-top:12px" data-resend>Made a mistake? Pay or send again</button>
      </div>
      <div class="card"><h3 class="card-title" style="margin-bottom:12px">${icon('phone')} Speed it up with a call</h3>${callBox(c)}</div>
    </div>
  </div>`;
}

function wireCheckout(c) {
  const view = document.getElementById('view');
  wireCopy(view);
  view.querySelectorAll('.seg button').forEach(b => b.addEventListener('click', () => { chosenSlot[c.id] = b.dataset.slot; viewEnroll(c.id); }));
  view.querySelectorAll('[data-next]').forEach(b => b.addEventListener('click', () => { enrollStep[c.id] = Number(b.dataset.next); viewEnroll(c.id); }));
  const again = view.querySelector('[data-resend]');
  if (again) again.addEventListener('click', () => { resend[c.id] = true; enrollStep[c.id] = 1; viewEnroll(c.id); });
  const form = document.getElementById('claimForm');
  if (form) form.addEventListener('submit', e => sendClaim(e, c));
}

async function sendClaim(e, c) {
  e.preventDefault();
  const f = e.target, err = f.querySelector('.error');
  const slotInput = f.querySelector('[name=qr_slot]:checked') || f.querySelector('[name=qr_slot]');
  const amount = f.amount.value.trim();
  showError(err, '');
  try {
    await withLoading(f.querySelector('button[type=submit]'), () => api('/api/me/claims', { method: 'POST', body: {
      course_id: c.id, qr_slot: slotInput ? slotInput.value : '',
      payer_name: f.payer_name.value, method: f.method.value,
      amount: amount === '' ? null : Number(amount),
      paid_on: f.paid_on.value, reference: f.reference.value,
    }}));
    showToast('Details sent. Now give us a quick call.', 'success');
    enrollStep[c.id] = 3;
    resend[c.id] = true; // stay on the wizard to show the call step
    await load();
    viewEnroll(c.id);
    resend[c.id] = false;
  } catch (ex) {
    showError(err, ex.message);
  }
}

// ---------- Updates ----------

function viewUpdates() {
  const canNotify = 'Notification' in window && Notification.permission === 'default' && hasActive();
  let body;
  if (!hasActive()) {
    body = emptyState('lock', 'Class links unlock after payment',
      'Google Meet links, the community group and instructions appear here once your course is active.',
      `<a class="btn primary" href="#/courses">${icon('graduation')} View courses</a>`);
  } else if (!updates || !updates.items.length) {
    body = emptyState('megaphone', 'No updates yet', 'Class links and announcements from the team will appear here.');
  } else {
    body = `<div id="updateList">${updates.items.map(u => updateCard(u, updates.seen_at)).join('')}</div>`;
  }
  setView(`
    <div class="view-head">
      <div><span class="kicker">Updates</span><h1>From G-CATION</h1><p>Class links, community and instructions for your courses.</p></div>
      ${canNotify ? `<button class="btn outline sm" id="notifyBtn">${icon('bell')} Alert me on this device</button>` : ''}
    </div>
    ${body}
  `);
  const nb = document.getElementById('notifyBtn');
  if (nb) nb.addEventListener('click', async () => {
    const result = await Notification.requestPermission();
    nb.remove();
    showToast(result === 'granted' ? 'Alerts on. You get a pop-up for new updates while the dashboard is open.' : 'Alerts are off. Updates still show here.',
      result === 'granted' ? 'success' : 'info');
  });
  markSeen();
}

// Opening the Updates page marks everything as read.
function markSeen() {
  if (seenMarked || !me.unread_announcements) return;
  seenMarked = true;
  api('/api/me/announcements/seen', { method: 'POST' }).then(() => {
    me.unread_announcements = 0;
    renderChrome();
  }).catch(() => { seenMarked = false; });
}

async function pollUpdates() {
  if (!me || !hasActive()) return;
  const fresh = await api('/api/me/announcements').catch(() => null);
  if (!fresh) return;
  const newOnes = fresh.items.filter(i => knownUpdateIds && !knownUpdateIds.has(i.id));
  knownUpdateIds = new Set(fresh.items.map(i => i.id));
  updates = fresh;
  if (!newOnes.length) return;
  seenMarked = false;
  me.unread_announcements += newOnes.length;
  renderChrome();
  const { name } = currentRoute();
  if (name === 'updates' || name === 'home' || name === '') route();
  const first = newOnes[0];
  showToast('New update: ' + first.title, 'success');
  if ('Notification' in window && Notification.permission === 'granted' && document.hidden) {
    const n = new Notification('G-CATION: ' + first.title, { body: first.message || first.link_label || 'Open your dashboard to see it.' });
    n.onclick = () => { window.focus(); go('updates'); };
  }
}

// ---------- Profile ----------

function viewProfile() {
  const s = me.student;
  setView(`
    <div class="view-head"><div><span class="kicker">Profile</span><h1>Your account</h1><p>Manage your details and password.</p></div></div>
    <div class="grid-2" style="align-items:start">
      <div class="stack">
        <div class="card pad-lg">
          <div class="row" style="gap:16px">
            <span class="avatar-sm avatar-lg">${esc(initials(s.name))}</span>
            <div style="min-width:0"><h2 style="margin:0;font-size:1.25rem">${esc(s.name)}</h2><div class="muted small">${esc(s.email)}</div>
              <div class="tiny faint">Member since ${fmtDate(s.created_at)}</div></div>
          </div>
          <div class="row" style="margin-top:16px"><span class="chip">Student ID <b class="mono">${s.id}</b> <button type="button" data-copy="${s.id}" data-copy-label="Student ID copied">Copy</button></span></div>
        </div>
        <form class="card pad-lg" id="profileForm" novalidate>
          <h3 class="card-title">${icon('user')} Personal details</h3>
          <div class="field"><label for="pName">Full name</label><input id="pName" name="name" maxlength="80" value="${esc(s.name)}" required></div>
          <div class="field"><label for="pPhone">Phone number</label><input id="pPhone" name="phone" type="tel" maxlength="20" value="${esc(s.phone)}"></div>
          <div class="field"><label>Email</label><input value="${esc(s.email)}" disabled><small>To change your email, contact the G-CATION team.</small></div>
          <div class="error"></div>
          <button class="btn primary" type="submit">${icon('check')} Save changes</button>
        </form>
      </div>
      <div class="stack">
        <div class="card pad-lg" id="receipts">
          <h3 class="card-title">${icon('download')} Payment receipts</h3>
          <p class="card-sub">A receipt is created automatically when your payment is approved.</p>
          ${me.receipts.length ? me.receipts.map(r => `<div class="receipt-row"><span class="ico">${icon('wallet')}</span>
              <div class="grow"><b>${esc(r.course_title)}</b><span><span class="mono">${esc(r.receipt_no)}</span> · ${fmtDate(r.granted_at)} · ${rupees(r.amount_paid)}${r.revoked_at ? ' · access revoked' : ''}</span></div>
              ${receiptButton(r, 'outline', 'PDF')}</div>`).join('')
            : '<p class="muted small" style="margin:0">No receipts yet. Your first one appears here after your payment is verified.</p>'}
        </div>
        <form class="card pad-lg" id="passwordForm" novalidate>
          <h3 class="card-title">${icon('key')} Change password</h3>
          <p class="card-sub">You stay logged in here. Other devices are logged out.</p>
          <div class="field"><label for="pCur">Current password</label><input id="pCur" name="current_password" type="password" autocomplete="current-password" required></div>
          <div class="field"><label for="pNew">New password</label><input id="pNew" name="new_password" type="password" autocomplete="new-password" minlength="8" required><small>At least 8 characters.</small></div>
          <div class="error"></div>
          <button class="btn primary" type="submit">${icon('key')} Update password</button>
        </form>
        <div class="card pad-lg">
          <h3 class="card-title">${icon('phone')} Need help?</h3>
          <p class="card-sub">Questions about payment, classes or your account.</p>
          ${[me.payment.contact_phone, HELP_PHONE_2].filter(Boolean).map(p => `<div class="row" style="margin-bottom:8px">
            <a class="btn outline" href="${esc(telHref(p))}">${icon('phone')} ${esc(p)}</a>
            <a class="btn outline" href="${esc(waHref(p, `Hi G-CATION, I need help. My Student ID is ${s.id}.`))}" target="_blank" rel="noopener">${icon('message')} WhatsApp</a></div>`).join('')}
          <div class="row"><a class="btn outline" href="mailto:${HELP_EMAIL}">${icon('mail')} ${HELP_EMAIL}</a></div>
        </div>
        <button class="btn danger block" id="logoutBtn2">${icon('logout')} Log out</button>
      </div>
    </div>
  `);
  wireCopy();
  wireReceipts();
  setupPasswordToggles();
  document.getElementById('logoutBtn2').addEventListener('click', logout);

  const pf = document.getElementById('profileForm');
  pf.addEventListener('submit', async e => {
    e.preventDefault();
    const err = pf.querySelector('.error');
    if (!pf.name.value.trim()) { showError(err, 'Enter your name.'); return; }
    showError(err, '');
    try {
      await withLoading(pf.querySelector('button[type=submit]'), () => api('/api/me', { method: 'PUT', body: { name: pf.name.value, phone: pf.phone.value } }));
      showToast('Profile saved.', 'success');
      refresh();
    } catch (ex) { showError(err, ex.message); }
  });

  const pw = document.getElementById('passwordForm');
  pw.addEventListener('submit', async e => {
    e.preventDefault();
    const err = pw.querySelector('.error');
    if (pw.new_password.value.length < 8) { showError(err, 'New password must be at least 8 characters.'); return; }
    showError(err, '');
    try {
      await withLoading(pw.querySelector('button[type=submit]'), () => api('/api/me/password', { method: 'POST', body: {
        current_password: pw.current_password.value, new_password: pw.new_password.value,
      }}));
      pw.reset();
      showToast('Password updated.', 'success');
    } catch (ex) { showError(err, ex.message); }
  });
}

// ---------- Boot ----------

async function logout() {
  await api('/api/logout', { method: 'POST' }).catch(() => {});
  setToken('student', '');
  location.href = '/';
}

document.querySelectorAll('[data-icon]').forEach(el => el.insertAdjacentHTML('afterbegin', icon(el.dataset.icon)));
document.getElementById('logoutBtn').innerHTML = icon('logout') + ' Log out';
document.getElementById('logoutBtn').addEventListener('click', logout);
window.addEventListener('hashchange', route);
route();
setInterval(pollUpdates, 60000);
document.addEventListener('visibilitychange', () => { if (!document.hidden) pollUpdates(); });
