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

// Demo/offline fallback data
const DEMO_DATA = {
  floodZones: {
    type: "FeatureCollection",
    features: [
      {
        type: "Feature",
        geometry: {
          type: "Polygon",
          coordinates: [[[107.605, -6.925],[107.615, -6.925],[107.615, -6.935],[107.605, -6.935],[107.605, -6.925]]]
        },
        properties: { id: 1, name: "Cikapundung Riverside", riskLevel: 4, source: "HISTORICAL", rainfallMm: 80 }
      },
      {
        type: "Feature",
        geometry: {
          type: "Polygon",
          coordinates: [[[107.655, -6.905],[107.665, -6.905],[107.665, -6.915],[107.655, -6.915],[107.655, -6.905]]]
        },
        properties: { id: 2, name: "Antapani Low Zone", riskLevel: 3, source: "HISTORICAL", rainfallMm: 55 }
      },
      {
        type: "Feature",
        geometry: {
          type: "Polygon",
          coordinates: [[[107.700, -6.940],[107.720, -6.940],[107.720, -6.955],[107.700, -6.955],[107.700, -6.940]]]
        },
        properties: { id: 3, name: "Gedebage Industrial Area", riskLevel: 3, source: "HISTORICAL", rainfallMm: 60 }
      }
    ]
  },

  incidents: {
    type: "FeatureCollection",
    features: [
      {
        type: "Feature",
        geometry: { type: "Point", coordinates: [107.6097, -6.9291] },
        properties: { id: 1, type: "FLOOD", severity: 4, title: "Banjir Cikapundung", description: "Jalan terendam setinggi 40cm", upvotes: 12, isVerified: true, reportedAt: new Date(Date.now() - 1800000).toISOString() }
      },
      {
        type: "Feature",
        geometry: { type: "Point", coordinates: [107.6483, -6.9132] },
        properties: { id: 2, type: "CONGESTION", severity: 3, title: "Macet Jl. Soekarno-Hatta", description: "Antrean panjang akibat genangan air", upvotes: 8, isVerified: false, reportedAt: new Date(Date.now() - 3600000).toISOString() }
      },
      {
        type: "Feature",
        geometry: { type: "Point", coordinates: [107.6322, -6.9205] },
        properties: { id: 3, type: "ROAD_CLOSURE", severity: 5, title: "Jalan Ditutup", description: "Jalan Laswi ditutup akibat longsor kecil", upvotes: 21, isVerified: true, reportedAt: new Date(Date.now() - 900000).toISOString() }
      },
      {
        type: "Feature",
        geometry: { type: "Point", coordinates: [107.6712, -6.9388] },
        properties: { id: 4, type: "FALLEN_TREE", severity: 2, title: "Pohon Tumbang", description: "Pohon menutupi sebagian jalur", upvotes: 3, isVerified: false, reportedAt: new Date(Date.now() - 7200000).toISOString() }
      }
    ]
  },

  weather: [
    { source: "OPENWEATHER", temperature: 23.5, humidity: 82, rainfall1h: 3.2, conditionText: "moderate rain", fetchedAt: new Date().toISOString() }
  ],

  generateMockRoutes(oLat, oLon, dLat, dLon) {
    const midLat = (oLat + dLat) / 2;
    const midLon = (oLon + dLon) / 2;
    const dist = Math.sqrt(Math.pow(dLat - oLat, 2) + Math.pow(dLon - oLon, 2)) * 111320;

    return {
      recommended: {
        id: 1, isRecommended: true,
        safetyScore: 0.82, congestionProb: 0.15, hazardCount: 1,
        distanceM: dist * 1.05, durationS: dist * 1.05 / 10,
        etaMinutes: dist * 1.05 / 600,
        geometry: {
          type: "LineString",
          coordinates: [
            [oLon, oLat], [midLon - 0.003, midLat + 0.002],
            [midLon, midLat], [dLon, dLat]
          ]
        },
        hazards: [{ type: "CONGESTION", severity: 2, lat: midLat, lon: midLon, description: "Slight congestion" }]
      },
      alternatives: [
        {
          id: 2, isRecommended: false,
          safetyScore: 0.55, congestionProb: 0.35, hazardCount: 3,
          distanceM: dist * 0.95, durationS: dist * 0.95 / 8,
          etaMinutes: dist * 0.95 / 480,
          geometry: {
            type: "LineString",
            coordinates: [
              [oLon, oLat], [midLon + 0.004, midLat - 0.003],
              [midLon + 0.002, midLat + 0.001], [dLon, dLat]
            ]
          },
          hazards: [
            { type: "FLOOD_ZONE", severity: 3, lat: midLat - 0.002, lon: midLon + 0.003, description: "Flood-prone area" },
            { type: "CONGESTION", severity: 3, lat: midLat, lon: midLon + 0.002, description: "Heavy traffic" }
          ]
        },
        {
          id: 3, isRecommended: false,
          safetyScore: 0.38, congestionProb: 0.60, hazardCount: 4,
          distanceM: dist * 1.15, durationS: dist * 1.15 / 6,
          etaMinutes: dist * 1.15 / 360,
          geometry: {
            type: "LineString",
            coordinates: [
              [oLon, oLat], [midLon - 0.006, midLat - 0.004],
              [midLon - 0.002, midLat - 0.002], [dLon, dLat]
            ]
          },
          hazards: [
            { type: "FLOOD", severity: 4, lat: midLat - 0.003, lon: midLon - 0.004, description: "Active flooding" },
            { type: "ROAD_CLOSURE", severity: 5, lat: midLat - 0.001, lon: midLon - 0.002, description: "Road closed" }
          ]
        }
      ]
    };
  }
};
