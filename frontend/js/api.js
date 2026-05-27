// ─── FloodRoute API Client ────────────────────────────────────────────────────

const API = (() => {
  const BASE = window.FLOODROUTE_API_BASE || '';  // same origin by default
  let _token = localStorage.getItem('fr_token');

  function authHeaders() {
    const h = { 'Content-Type': 'application/json' };
    if (_token) h['Authorization'] = `Bearer ${_token}`;
    return h;
  }

  async function request(method, path, body = null) {
    const opts = { method, headers: authHeaders() };
    if (body) opts.body = JSON.stringify(body);
    const res = await fetch(`${BASE}${path}`, opts);
    if (!res.ok) {
      let msg = `HTTP ${res.status}`;
      try { const e = await res.json(); msg = e.message || msg; } catch (_) {}
      throw new Error(msg);
    }
    return res.json();
  }

  return {
    // ─── Auth ─────────────────────────────────────────────────────────────
    setToken(t) { _token = t; localStorage.setItem('fr_token', t); },
    clearToken() { _token = null; localStorage.removeItem('fr_token'); },
    getToken() { return _token; },

    async login(username, password) {
      const data = await request('POST', '/api/auth/login', { username, password });
      API.setToken(data.token);
      return data;
    },

    async register(username, email, password, role = 'CONSUMER') {
      return request('POST', '/api/auth/register', { username, email, password, role });
    },

    async getPendingUsers() {
      return request('GET', '/api/admin/users/pending');
    },

    async approveUser(id, role) {
      return request('PATCH', `/api/admin/users/${id}/approve`, { role });
    },

    // ─── Routes ───────────────────────────────────────────────────────────
    async getRoutes(originLat, originLon, destLat, destLon, profile = 'driving-car') {
      return request('POST', '/api/routes', {
        originLat, originLon, destLat, destLon, profile, alternatives: 2
      });
    },

    // ─── Incidents ────────────────────────────────────────────────────────
    async getIncidents(lat, lon, radius = 5000) {
      let url = '/api/incidents?format=geojson';
      if (lat && lon) url += `&lat=${lat}&lon=${lon}&radius=${radius}`;
      return request('GET', url);
    },

    async submitIncident(type, severity, latitude, longitude, description = '', imageUrl = null) {
      return request('POST', '/api/incidents', {
        type, severity, latitude, longitude, description: description || null, imageUrl
      });
    },

    async uploadMedia(file) {
      const base64Data = await fileToBase64(file);
      return request('POST', '/api/media', {
        filename: file.name,
        contentType: file.type,
        base64Data
      });
    },

    async upvoteIncident(id) {
      return request('POST', `/api/incidents/${id}/upvote`);
    },

    // ─── Flood Zones ──────────────────────────────────────────────────────
    async getFloodZones() {
      return request('GET', '/api/flood-zones');
    },

    // ─── Weather ──────────────────────────────────────────────────────────
    async getWeather() {
      return request('GET', '/api/weather');
    }
  };

  function fileToBase64(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = () => reject(new Error('Could not read selected file'));
      reader.readAsDataURL(file);
    });
  }
})();

