// Sign-in box for students and management: log in with a mobile number (or, for older accounts, the
// email they registered with) and password, register with name, mobile number and password, or
// continue with Google. The page sets AUTH = { role: 'student' | 'management', home: '...' } first.
// (The admin has its own fixed login on admin-login.html.)

const isTeam = AUTH.role === 'management';
const base = isTeam ? '/api/management' : '/api';
let mode = 'login';        // 'login' | 'register'
let googleCredential = ''; // kept so a new person can add their mobile (and referral code) and continue

const TEXT = {
  login: ['Welcome back', isTeam ? 'Log in to see courses and your Management ID.' : 'Log in to see your courses and updates.'],
  register: [isTeam ? 'Join the team' : 'Create your account', isTeam ? 'You need the referral code from the G-CATION admin.' : 'It takes less than a minute.'],
};
// '+91 98765-43210' or '098765 43210' -> '9876543210'.
const mobileDigits = v => v.replace(/\D/g, '').replace(/^(91|0)(?=\d{10}$)/, '');

function renderAuth() {
  document.getElementById('authBox').innerHTML = `
    <span class="kicker">${isTeam ? icon('shield') + ' Management team' : 'Student access'}</span>
    <h1 id="title"></h1>
    <p class="muted small" id="subtitle" style="margin:0"></p>
    <div class="tabs" role="tablist" id="tabs">
      <button class="tab" type="button" role="tab" data-mode="login">Log in</button>
      <button class="tab" type="button" role="tab" data-mode="register">Register</button>
    </div>

    <form id="loginForm" novalidate>
      <div class="field"><label for="lUser">Mobile number</label><input id="lUser" type="text" inputmode="tel" autocomplete="username" autocapitalize="none" spellcheck="false" maxlength="120" placeholder="10-digit mobile number" required>
        <small>Registered earlier with an email? You can type that email here.</small></div>
      <div class="field"><label for="lPass">Password</label><input id="lPass" type="password" autocomplete="current-password" required></div>
      <div class="error"></div>
      <button class="btn primary block lg" type="submit" style="margin-top:6px">Log in</button>
      <p class="muted small center" style="margin:14px 0 0">Forgot your password? ${isTeam ? 'Contact the G-CATION admin.' : 'Call the G-CATION team and they will reset it for you.'}</p>
    </form>

    <form id="registerForm" hidden novalidate>
      <div class="field"><label for="rName">Full name</label><input id="rName" autocomplete="name" maxlength="80" placeholder="Your full name" required></div>
      <div class="field"><label for="rPhone">Mobile number</label>
        <div class="input-addon"><span class="addon">+91</span><input id="rPhone" type="tel" inputmode="numeric" autocomplete="tel-national" maxlength="14" placeholder="10-digit mobile number" required></div>
        <small>You log in with this number.</small></div>
      <div class="field"><label for="rPass">Password</label><input id="rPass" type="password" autocomplete="new-password" minlength="8" required><small>At least 8 characters.</small></div>
      ${isTeam ? `<div class="field"><label for="rCode">Referral code</label><input id="rCode" maxlength="60" autocomplete="off" spellcheck="false" required><small>The code the G-CATION admin gave you.</small></div>` : ''}
      <div class="error"></div>
      <button class="btn primary block lg" type="submit" style="margin-top:6px">${isTeam ? 'Create management account' : 'Create account'}</button>
      ${isTeam ? '' : '<p class="muted tiny center" style="margin:12px 0 0">Registration is free. You only pay when you enroll in a course.</p>'}
    </form>

    <div id="googleBox" hidden>
      <div class="or-line"><span>or</span></div>
      <div id="googleBtn" class="google-btn"></div>
      <div id="detailsBox" class="google-details" hidden>
        <h3 style="margin:0 0 4px;font-size:1.05rem">One more step</h3>
        <p class="muted small" style="margin:0 0 8px" id="gWho"></p>
        <div class="field"><label for="gMobile">Mobile number</label>
          <div class="input-addon"><span class="addon">+91</span><input id="gMobile" type="tel" inputmode="numeric" autocomplete="tel-national" maxlength="14" placeholder="10-digit mobile number"></div>
          <small>You can also log in with this number.</small></div>
        ${isTeam ? `<div class="field"><label for="gCode">Referral code</label><input id="gCode" maxlength="60" autocomplete="off" spellcheck="false"><small>The code the G-CATION admin gave you.</small></div>` : ''}
        <button type="button" class="btn primary block" id="gContinue">${isTeam ? 'Create management account' : 'Create account'}</button>
      </div>
      <div class="error" id="gErr"></div>
      <p class="muted tiny center" style="margin:10px 0 0">Registered with your Gmail before? Use the same Google account and your courses and data stay as they are.</p>
    </div>`;
  document.querySelectorAll('[data-mode]').forEach(b => b.addEventListener('click', () => setMode(b.dataset.mode)));
  document.getElementById('loginForm').addEventListener('submit', login);
  document.getElementById('registerForm').addEventListener('submit', register);
  document.getElementById('gContinue').addEventListener('click', completeGoogle);
  setupPasswordToggles();
}

function setMode(m) {
  mode = m;
  document.getElementById('loginForm').hidden = m !== 'login';
  document.getElementById('registerForm').hidden = m !== 'register';
  document.querySelectorAll('#tabs .tab').forEach(t => t.classList.toggle('on', t.dataset.mode === m));
  document.getElementById('title').textContent = TEXT[m][0];
  document.getElementById('subtitle').textContent = TEXT[m][1];
  document.getElementById('googleBox').hidden = !window.GOOGLE_CLIENT_ID;
  document.querySelectorAll('.error').forEach(e => showError(e, ''));
  history.replaceState(null, '', m === 'register' ? '#register' : '#login');
}

// Keeps the login token (api() only does that by itself for /login and /register) and opens the app.
function signedIn(res) {
  if (res && res.token) setToken(AUTH.role, res.token);
  location.href = AUTH.home;
}

async function done(form, path, body) {
  const err = form.querySelector('.error');
  try {
    signedIn(await withLoading(form.querySelector('button[type=submit]'), () => api(path, { method: 'POST', body: { ...body, client: 'app' } })));
  } catch (e) { showError(err, e.message); }
}

function problem(form, rules) {
  const msg = (rules.find(([ok]) => !ok) || [])[1] || '';
  showError(form.querySelector('.error'), msg);
  return msg;
}

function login(e) {
  e.preventDefault();
  const f = e.target;
  const login = document.getElementById('lUser').value.trim(), password = document.getElementById('lPass').value;
  const isEmail = login.includes('@');
  if (problem(f, [
    [isEmail || /^[6-9]\d{9}$/.test(mobileDigits(login)), 'Enter your 10-digit mobile number.'],
    [password, 'Enter your password.'],
  ])) return;
  done(f, base + '/login', isEmail ? { email: login, password } : { mobile: mobileDigits(login), password });
}

function register(e) {
  e.preventDefault();
  const f = e.target;
  const body = {
    name: document.getElementById('rName').value.trim(), mobile: mobileDigits(document.getElementById('rPhone').value),
    password: document.getElementById('rPass').value,
  };
  if (isTeam) body.invite_code = document.getElementById('rCode').value.trim();
  if (problem(f, [
    [body.name, 'Enter your full name.'],
    [/^[6-9]\d{9}$/.test(body.mobile), 'Enter a valid 10-digit mobile number.'],
    [body.password.length >= 8, 'Password must be at least 8 characters.'],
    [!isTeam || body.invite_code, 'Enter the referral code from the G-CATION admin.'],
  ])) return;
  done(f, base + '/register', body);
}

// ---------- Google ----------

// Signs in with the ID token from Google's button. People new to G-CATION get a 428: they add their
// mobile number (and, for management, the referral code), then the same token is sent again.
async function googleSignIn(credential, extra = {}) {
  const err = document.getElementById('gErr');
  showError(err, '');
  if (!credential) return;
  googleCredential = credential;
  try {
    signedIn(await api('/api/auth/google', { method: 'POST', body: { credential, role: AUTH.role, client: 'app', ...extra } }));
  } catch (e) {
    if (e.status === 428) {
      const box = document.getElementById('detailsBox');
      box.hidden = false;
      document.getElementById('gWho').textContent = e.message;
      document.getElementById('gMobile').focus();
      return;
    }
    showError(err, e.message);
  }
}

function completeGoogle() {
  const err = document.getElementById('gErr');
  const mobile = mobileDigits(document.getElementById('gMobile').value);
  const invite = isTeam ? document.getElementById('gCode').value.trim() : '';
  if (!/^[6-9]\d{9}$/.test(mobile)) { showError(err, 'Enter a valid 10-digit mobile number.'); return; }
  if (isTeam && !invite) { showError(err, 'Enter the referral code from the G-CATION admin.'); return; }
  withLoading(document.getElementById('gContinue'), () => googleSignIn(googleCredential, { mobile, invite_code: invite }));
}

// Google draws its own button; it is redrawn to match the theme and the width of the box.
function renderGoogleButton() {
  const box = document.getElementById('googleBtn');
  if (!window.google || !google.accounts || box.closest('[hidden]')) return;
  box.innerHTML = '';
  google.accounts.id.renderButton(box, {
    theme: currentTheme() === 'dark' ? 'filled_black' : 'outline', size: 'large', shape: 'pill',
    text: 'continue_with', logo_alignment: 'center', width: Math.min(box.clientWidth || 320, 400),
  });
}

function loadGoogle() {
  if (!window.GOOGLE_CLIENT_ID) return;
  const s = document.createElement('script');
  s.src = 'https://accounts.google.com/gsi/client';
  s.async = true;
  s.onload = () => {
    google.accounts.id.initialize({ client_id: window.GOOGLE_CLIENT_ID, callback: r => googleSignIn(r.credential) });
    renderGoogleButton();
    // Follow the sun/moon button and the device theme.
    new MutationObserver(renderGoogleButton).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', renderGoogleButton);
    let t;
    addEventListener('resize', () => { clearTimeout(t); t = setTimeout(renderGoogleButton, 200); });
  };
  document.head.appendChild(s);
}

renderAuth();
setMode(location.hash === '#register' ? 'register' : 'login');
api(base + '/me').then(() => location.replace(AUTH.home)).catch(() => {});
api('/api/auth/config').then(c => {
  window.GOOGLE_CLIENT_ID = c.google_client_id;
  setMode(mode);
  loadGoogle();
}).catch(() => {});
