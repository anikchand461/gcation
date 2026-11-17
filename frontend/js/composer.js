// Update composer shared by the admin dashboard and management members the admin lets post updates:
// write an update, choose who sees it, preview it, and edit or delete posted ones.

let composer = { base: '', login: '' };
let audience = {};
let annCourses = [];
let editingAnn = null;

async function composerCall(fn) {
  try { return await fn(); }
  catch (e) {
    if (e.status === 401) { location.replace(composer.login); return new Promise(() => {}); }
    showToast(e.message, 'error');
    throw e;
  }
}

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

// Opens the composer page. `heading` is the page title HTML, `base` the API path for the updates
// (GET lists them with courses and audience counts; POST, PUT /id and DELETE /id change them).
async function composerView({ heading, base, login }) {
  composer = { base, login };
  editingAnn = null;
  setView(`
    ${heading}
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
        <div id="annList"><div class="skeleton" style="height:200px"></div></div>
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
    ...annCourses.map(c => ['c' + c.id, 'book', `${c.title} students`, audience[c.id]])];
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
  showError(err, '');
  if (!editingAnn && !await confirmDialog({
    title: 'Post this update?', message: `${audienceText()} will see it right away.`, confirmText: 'Post update',
  })) return;
  try {
    await withLoading(document.getElementById('annSubmit'), () => editingAnn
      ? api(`${composer.base}/${editingAnn.id}`, { method: 'PUT', body })
      : api(composer.base, { method: 'POST', body }));
    showToast(editingAnn ? 'Update saved.' : 'Update posted.', 'success');
    resetComposer();
    loadAnnouncements();
  } catch (ex) {
    if (ex.status === 401) return location.replace(composer.login);
    showError(err, ex.message);
  }
}

async function loadAnnouncements() {
  const data = await composerCall(() => api(composer.base));
  audience = data.audience;
  annCourses = data.courses;
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
    <div class="tiny muted">${fmtDateTime(a.created_at)}${a.updated_at > a.created_at + 60 ? ' · edited' : ''}${a.sent_by ? ` · posted by <b>${esc(a.sent_by)}</b>` : ''}</div>
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
    await composerCall(() => api(`${composer.base}/${b.dataset.del}`, { method: 'DELETE' }));
    if (editingAnn && editingAnn.id === Number(b.dataset.del)) resetComposer();
    showToast('Update deleted.', 'success');
    loadAnnouncements();
  }));
}
