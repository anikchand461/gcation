// Address of the FastAPI backend. Change this to your live API address when you deploy,
// e.g. 'https://api.gcation.in'.
const API_BASE = 'http://127.0.0.1:8000';
// const API_BASE = 'https://gcation.onrender.com';

// Browsers treat localhost and 127.0.0.1 as different sites and will not send the login cookie
// between them. When developing locally, always use the same host name as API_BASE.
if (location.hostname === 'localhost' && API_BASE.includes('//127.0.0.1')) {
  location.replace(location.href.replace('//localhost', '//127.0.0.1'));
}
