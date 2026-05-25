// ─── FloodRoute App Controller ────────────────────────────────────────────────

(function () {
  'use strict';

  // ─── State ───────────────────────────────────────────────────────────────
  const state = {
    origin: null,        // { lat, lon, label }
    dest: null,          // { lat, lon, label }
    profile: 'driving-car',
    clickMode: null,     // 'origin' | 'dest' | 'report'
    routeData: null,
    selectedRouteIdx: 0,
    reportCoords: null,
    reportType: 'FLOOD',
    reportSeverity: 2,
    user: null,
    useDemoData: false   // use the live backend by default
  };

  // ─── DOM refs ─────────────────────────────────────────────────────────────
  const $ = id => document.getElementById(id);
  const $inputOrigin   = $('inputOrigin');
  const $inputDest     = $('inputDest');
  const $coordOrigin   = $('coordOrigin');
  const $coordDest     = $('coordDest');
  const $btnRoute      = $('btnRoute');
  const $btnLocate     = $('btnLocate');
  const $btnSwap       = $('btnSwap');
  const $routeResults  = $('routeResults');
  const $routeCards    = $('routeCards');
  const $resultsCount  = $('resultsCount');
  const $incidentList  = $('incidentList');
  const $mapLoading    = $('mapLoading');
  const $clickMode     = $('clickMode');
  const $clickModeText = $('clickModeText');
  const $reportFeedback = $('reportFeedback');
  const $reportCoords  = $('reportCoords');
  const $severitySlider  = $('severitySlider');
  const $severityDisplay = $('severityDisplay');
  const $weatherTemp   = $('weatherTemp');
  const $weatherRain   = $('weatherRain');
  const $navAdmin      = $('navAdmin');
  const $accountChip   = $('accountChip');
  const $approvalList  = $('approvalList');

  // ─── Init ─────────────────────────────────────────────────────────────────

  function init() {
    const map = MapManager.init([-6.9175, 107.6191], 13);

    // Map click handler
    map.on('click', onMapClick);

    // Wire up panels
    document.querySelectorAll('.nav-btn').forEach(btn => {
      btn.addEventListener('click', () => switchPanel(btn.dataset.panel));
    });

    // Route controls
    $btnLocate.addEventListener('click', locateMe);
    $btnSwap.addEventListener('click', swapOriginDest);
    $btnRoute.addEventListener('click', calculateRoute);

    // Origin/dest inputs - click on map mode
    $inputOrigin.addEventListener('click', () => enterClickMode('origin'));
    $inputDest.addEventListener('click', () => enterClickMode('dest'));

    // Profile chips
    document.querySelectorAll('.chip[data-profile]').forEach(c => {
      c.addEventListener('click', () => {
        document.querySelectorAll('.chip[data-profile]').forEach(x => x.classList.remove('active'));
        c.classList.add('active');
        state.profile = c.dataset.profile;
      });
    });

    // Cancel click mode
    $('btnCancelClick').addEventListener('click', cancelClickMode);

    // Report panel
    document.querySelectorAll('.type-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        document.querySelectorAll('.type-btn').forEach(x => x.classList.remove('active'));
        btn.classList.add('active');
        state.reportType = btn.dataset.type;
      });
    });

    $severitySlider.addEventListener('input', () => {
      state.reportSeverity = parseInt($severitySlider.value);
      const labels = ['', 'Minor', 'Moderate', 'Significant', 'Severe', 'Extreme'];
      $severityDisplay.textContent = `${state.reportSeverity} – ${labels[state.reportSeverity]}`;
    });

    $('btnReportLocate').addEventListener('click', () => {
      navigator.geolocation?.getCurrentPosition(pos => {
        state.reportCoords = { lat: pos.coords.latitude, lon: pos.coords.longitude };
        $reportCoords.textContent = `${state.reportCoords.lat.toFixed(5)}, ${state.reportCoords.lon.toFixed(5)}`;
      }, () => toast('Could not get location', 'error'));
    });

    $('btnSubmitReport').addEventListener('click', submitReport);

    // Layer toggles
    ['FloodZones', 'Incidents', 'Routes', 'Weather'].forEach(name => {
      const key = name.charAt(0).toLowerCase() + name.slice(1);
      const toggle = $(`layer${name}`);
      toggle?.addEventListener('change', () => {
        MapManager.setLayerVisible(key === 'weather' ? 'weather' : key, toggle.checked);
      });
    });

    // Map controls
    $('btnZoomIn').addEventListener('click', () => MapManager.getMap().zoomIn());
    $('btnZoomOut').addEventListener('click', () => MapManager.getMap().zoomOut());
    $('btnFitBounds').addEventListener('click', () => MapManager.fitToAll());
    $('btnRefresh').addEventListener('click', loadAllData);

    // Auth
    $('btnAuth').addEventListener('click', () => $('authModal').classList.remove('hidden'));
    $('btnCloseAuth').addEventListener('click', () => $('authModal').classList.add('hidden'));
    document.querySelectorAll('.modal-tab').forEach(t => {
      t.addEventListener('click', () => switchAuthTab(t.dataset.tab));
    });
    $('loginForm').addEventListener('submit', handleLogin);
    $('registerForm').addEventListener('submit', handleRegister);
    $('btnLoadPending').addEventListener('click', loadPendingUsers);
    $accountChip.addEventListener('click', logout);

    // Close modal on overlay click
    $('authModal').addEventListener('click', e => {
      if (e.target === $('authModal')) $('authModal').classList.add('hidden');
    });

    // Load initial data
    restoreSessionLabel();
    loadAllData();

    // Refresh weather every 5 min
    setInterval(loadWeather, 300000);
  }

  // ─── Data Loading ─────────────────────────────────────────────────────────

  async function loadAllData() {
    await Promise.all([loadFloodZones(), loadIncidents(), loadWeather()]);
  }

  async function loadFloodZones() {
    try {
      let data;
      if (state.useDemoData) {
        data = DEMO_DATA.floodZones;
      } else if (!API.getToken()) {
        data = emptyFeatureCollection();
      } else {
        data = await API.getFloodZones();
      }
      MapManager.renderFloodZones(data);
    } catch (e) {
      MapManager.renderFloodZones(DEMO_DATA.floodZones);
    }
  }

  async function loadIncidents() {
    try {
      let data;
      if (state.useDemoData) {
        data = DEMO_DATA.incidents;
      } else if (!API.getToken()) {
        data = emptyFeatureCollection();
      } else {
        const center = MapManager.getMap().getCenter();
        data = await API.getIncidents(center.lat, center.lng, 10000);
      }
      MapManager.renderIncidents(data);
      renderIncidentSidebar(data.features || []);
    } catch (e) {
      MapManager.renderIncidents(DEMO_DATA.incidents);
      renderIncidentSidebar(DEMO_DATA.incidents.features);
    }
  }

  async function loadWeather() {
    try {
      let data;
      if (state.useDemoData) {
        data = DEMO_DATA.weather;
      } else if (!API.getToken()) {
        data = [];
      } else {
        data = await API.getWeather();
      }
      if (data && data.length > 0) {
        const w = data[0];
        $weatherTemp.textContent = w.temperature ? `${w.temperature.toFixed(1)}°C` : '--°C';
        $weatherRain.textContent = w.rainfall1h != null ? `${w.rainfall1h.toFixed(1)} mm/h` : '-- mm/h';
        updateWeatherIcon(w);
      }
    } catch (_) {}
  }

  function updateWeatherIcon(w) {
    const chip = $('weatherChip');
    const icon = chip.querySelector('.weather-icon');
    if (!icon) return;
    const rain = w.rainfall1h || 0;
    if (rain > 10) icon.textContent = '⛈️';
    else if (rain > 3) icon.textContent = '🌧️';
    else if (rain > 0) icon.textContent = '🌦️';
    else icon.textContent = '🌤️';
  }

  // ─── Route Calculation ────────────────────────────────────────────────────

  async function calculateRoute() {
    if (!state.useDemoData && !API.getToken()) {
      toast('Login is required to calculate routes', 'error');
      $('authModal').classList.remove('hidden');
      return;
    }
    if (!state.origin) { toast('Set an origin point', 'error'); enterClickMode('origin'); return; }
    if (!state.dest)   { toast('Set a destination point', 'error'); enterClickMode('dest'); return; }

    $mapLoading.classList.remove('hidden');
    $btnRoute.disabled = true;

    try {
      let data;
      if (state.useDemoData) {
        await sleep(800); // simulate network
        data = DEMO_DATA.generateMockRoutes(
          state.origin.lat, state.origin.lon,
          state.dest.lat, state.dest.lon
        );
      } else {
        data = await API.getRoutes(
          state.origin.lat, state.origin.lon,
          state.dest.lat, state.dest.lon,
          state.profile
        );
      }

      state.routeData = data;
      state.selectedRouteIdx = 0;

      MapManager.renderRoutes(data, 0);
      MapManager.fitToRoutes(data);
      renderRouteCards(data);

      $routeResults.classList.remove('hidden');
      toast(`✅ Found ${1 + (data.alternatives?.length || 0)} routes`, 'success');
    } catch (e) {
      toast('Route calculation failed: ' + e.message, 'error');
    } finally {
      $mapLoading.classList.add('hidden');
      $btnRoute.disabled = false;
    }
  }

  function renderRouteCards(data) {
    $routeCards.innerHTML = '';
    const all = [data.recommended, ...(data.alternatives || [])];
    $resultsCount.textContent = `${all.length} found`;

    all.forEach((route, idx) => {
      const score = route.safetyScore;
      const scoreClass = score >= 0.8 ? 'safe' : score >= 0.5 ? 'warn' : 'danger';
      const dist = (route.distanceM / 1000).toFixed(1);
      const eta = Math.ceil(route.etaMinutes);

      const card = document.createElement('div');
      card.className = `route-card ${scoreClass}${idx === 0 ? ' selected' : ''}`;

      const hazardChips = (route.hazards || []).slice(0, 3).map(h =>
        `<span class="hazard-chip">${h.type.replace(/_/g,' ')}</span>`
      ).join('');

      card.innerHTML = `
        <div class="route-card-header">
          <span class="route-badge ${idx === 0 ? 'recommended' : 'alternative'}">
            ${idx === 0 ? '★ Safest' : `Alt ${idx}`}
          </span>
          <span class="safety-score ${scoreClass}">${(score * 100).toFixed(0)}%</span>
        </div>
        <div class="route-stats">
          <div class="stat">
            <span class="stat-label">Distance</span>
            <span class="stat-value">${dist} km</span>
          </div>
          <div class="stat">
            <span class="stat-label">ETA</span>
            <span class="stat-value">${eta} min</span>
          </div>
          <div class="stat">
            <span class="stat-label">Hazards</span>
            <span class="stat-value">${route.hazardCount}</span>
          </div>
          <div class="stat">
            <span class="stat-label">Congestion</span>
            <span class="stat-value">${(route.congestionProb * 100).toFixed(0)}%</span>
          </div>
        </div>
        ${hazardChips ? `<div class="hazard-chips">${hazardChips}</div>` : ''}
      `;

      card.addEventListener('click', () => {
        document.querySelectorAll('.route-card').forEach(c => c.classList.remove('selected'));
        card.classList.add('selected');
        state.selectedRouteIdx = idx;
        MapManager.renderRoutes(data, idx);
      });

      $routeCards.appendChild(card);
    });
  }

  // ─── Incident Sidebar ─────────────────────────────────────────────────────

  function renderIncidentSidebar(features) {
    $incidentList.innerHTML = '';
    if (!features?.length) {
      $incidentList.innerHTML = '<p style="color:var(--text-muted);font-size:12px">No active incidents nearby</p>';
      return;
    }

    features.slice(0, 5).forEach(f => {
      const p = f.properties;
      const sev = p.severity || 1;
      const sevDots = Array.from({ length: 5 }, (_, i) => {
        const active = i < sev ? ` active-${sev}` : '';
        return `<div class="sev-dot${active}"></div>`;
      }).join('');

      const when = p.reportedAt ? timeAgo(new Date(p.reportedAt)) : '';

      const el = document.createElement('div');
      el.className = 'incident-item';
      el.innerHTML = `
        <div class="incident-type-icon">${MapManager.incidentEmoji(p.type)}</div>
        <div class="incident-info">
          <div class="incident-type-label">${MapManager.formatIncidentType(p.type)}</div>
          ${p.title ? `<div style="font-size:11px;color:var(--text-secondary)">${p.title}</div>` : ''}
          <div class="incident-meta">${when}</div>
          <div class="incident-severity">${sevDots}</div>
        </div>
      `;
      $incidentList.appendChild(el);
    });
  }

  // ─── Click Mode ───────────────────────────────────────────────────────────

  function enterClickMode(mode) {
    state.clickMode = mode;
    $clickMode.style.display = 'flex';
    $clickModeText.textContent = mode === 'origin'
      ? 'Click map to set origin point'
      : mode === 'dest'
        ? 'Click map to set destination'
        : 'Click map to place incident';
    MapManager.getMap().getContainer().style.cursor = 'crosshair';
  }

  function cancelClickMode() {
    state.clickMode = null;
    $clickMode.style.display = 'none';
    MapManager.getMap().getContainer().style.cursor = '';
  }

  function onMapClick(e) {
    const { lat, lng: lon } = e.latlng;
    if (!state.clickMode) return;

    if (state.clickMode === 'origin') {
      state.origin = { lat, lon };
      $inputOrigin.value = `${lat.toFixed(5)}, ${lon.toFixed(5)}`;
      $coordOrigin.textContent = `${lat.toFixed(6)}, ${lon.toFixed(6)}`;
      MapManager.setOrigin(lat, lon);
    } else if (state.clickMode === 'dest') {
      state.dest = { lat, lon };
      $inputDest.value = `${lat.toFixed(5)}, ${lon.toFixed(5)}`;
      $coordDest.textContent = `${lat.toFixed(6)}, ${lon.toFixed(6)}`;
      MapManager.setDest(lat, lon);
    } else if (state.clickMode === 'report') {
      state.reportCoords = { lat, lon };
      $reportCoords.textContent = `${lat.toFixed(5)}, ${lon.toFixed(5)}`;
    }

    cancelClickMode();
  }

  // ─── Geolocation ──────────────────────────────────────────────────────────

  function locateMe() {
    if (!navigator.geolocation) { toast('Geolocation not supported', 'error'); return; }
    navigator.geolocation.getCurrentPosition(
      pos => {
        const { latitude: lat, longitude: lon } = pos.coords;
        state.origin = { lat, lon };
        $inputOrigin.value = `My Location (${lat.toFixed(4)}, ${lon.toFixed(4)})`;
        $coordOrigin.textContent = `${lat.toFixed(6)}, ${lon.toFixed(6)}`;
        MapManager.setOrigin(lat, lon);
        MapManager.getMap().setView([lat, lon], 14);
        toast('📍 Location set', 'success');
      },
      () => toast('Location access denied', 'error'),
      { enableHighAccuracy: true, timeout: 8000 }
    );
  }

  function swapOriginDest() {
    [state.origin, state.dest] = [state.dest, state.origin];
    [$inputOrigin.value, $inputDest.value] = [$inputDest.value, $inputOrigin.value];
    [$coordOrigin.textContent, $coordDest.textContent] = [$coordDest.textContent, $coordOrigin.textContent];
    if (state.origin) MapManager.setOrigin(state.origin.lat, state.origin.lon);
    if (state.dest) MapManager.setDest(state.dest.lat, state.dest.lon);
  }

  // ─── Report Submission ────────────────────────────────────────────────────

  async function submitReport() {
    if (!state.useDemoData && !API.getToken()) {
      showReportFeedback('Login as a producer to submit reports', 'error');
      $('authModal').classList.remove('hidden');
      return;
    }
    if (!state.reportCoords) {
      toast('Set a location for the report', 'error');
      enterClickMode('report');
      return;
    }

    const desc = $('reportDesc').value.trim();
    const mediaFile = $('reportMedia').files?.[0] || null;

    try {
      if (state.useDemoData) {
        await sleep(500);
        showReportFeedback('✅ Report submitted successfully! Thank you.', 'success');
        toast('Report submitted', 'success');
        await loadIncidents();
      } else {
        let imageUrl = null;
        if (mediaFile) {
          const media = await API.uploadMedia(mediaFile);
          imageUrl = media.url;
        }
        await API.submitIncident(
          state.reportType,
          state.reportSeverity,
          state.reportCoords.lat,
          state.reportCoords.lon,
          desc,
          imageUrl
        );
        showReportFeedback('✅ Report submitted successfully!', 'success');
        toast('Report submitted', 'success');
        await loadIncidents();
      }
    } catch (e) {
      showReportFeedback('❌ ' + e.message, 'error');
    }
  }

  function showReportFeedback(msg, type) {
    $reportFeedback.className = `report-feedback ${type}`;
    $reportFeedback.textContent = msg;
    $reportFeedback.classList.remove('hidden');
    setTimeout(() => $reportFeedback.classList.add('hidden'), 4000);
  }

  // ─── Panel Switching ──────────────────────────────────────────────────────

  function switchPanel(name) {
    document.querySelectorAll('.nav-btn').forEach(b =>
      b.classList.toggle('active', b.dataset.panel === name)
    );
    document.querySelectorAll('.panel').forEach(p =>
      p.classList.toggle('active', p.id === `panel${name.charAt(0).toUpperCase() + name.slice(1)}`)
    );
  }

  // ─── Auth ─────────────────────────────────────────────────────────────────

  function switchAuthTab(tab) {
    document.querySelectorAll('.modal-tab').forEach(t =>
      t.classList.toggle('active', t.dataset.tab === tab)
    );
    $('loginForm').classList.toggle('hidden', tab !== 'login');
    $('registerForm').classList.toggle('hidden', tab !== 'register');
  }

  async function handleLogin(e) {
    e.preventDefault();
    const username = $('loginUsername').value.trim();
    const password = $('loginPassword').value;
    const errEl = $('loginError');
    errEl.classList.add('hidden');

    try {
      const data = await API.login(username, password);
      state.user = data;
      localStorage.setItem('fr_user', JSON.stringify(data));
      updateAuthState();
      $('authModal').classList.add('hidden');
      toast(`👋 Welcome back, ${data.displayName || data.username}!`, 'success');
      await loadAllData();
      if (data.role === 'SUPERADMIN') await loadPendingUsers();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  }

  async function handleRegister(e) {
    e.preventDefault();
    const username = $('regUsername').value.trim();
    const email    = $('regEmail').value.trim();
    const password = $('regPassword').value;
    const role     = $('regRole').value;
    const errEl    = $('registerError');
    errEl.classList.add('hidden');

    try {
      const data = await API.register(username, email, password, role);
      $('authModal').classList.add('hidden');
      toast(data.message || 'Registration submitted for approval', 'success');
      e.target.reset();
    } catch (err) {
      errEl.textContent = err.message;
      errEl.classList.remove('hidden');
    }
  }
  // Toast ────────────────────────────────────────────────────────────────

  function restoreSessionLabel() {
    try {
      const stored = localStorage.getItem('fr_user');
      if (stored && API.getToken()) state.user = JSON.parse(stored);
    } catch (_) {
      state.user = null;
    }
    updateAuthState();
  }

  function updateAuthState() {
    const loggedIn = !!state.user;
    $('btnAuth').classList.toggle('hidden', loggedIn);
    $accountChip.classList.toggle('hidden', !loggedIn);
    $navAdmin.classList.toggle('hidden', state.user?.role !== 'SUPERADMIN');
    if (loggedIn) {
      $accountChip.textContent = `${state.user.username} - ${formatRole(state.user.role)}`;
    }
  }

  function logout() {
    API.clearToken();
    localStorage.removeItem('fr_user');
    state.user = null;
    updateAuthState();
    switchPanel('route');
    toast('Signed out', 'info');
  }

  async function loadPendingUsers() {
    if (state.user?.role !== 'SUPERADMIN') return;
    $approvalList.innerHTML = '<p class="hint-text">Loading pending users...</p>';
    try {
      const users = await API.getPendingUsers();
      renderPendingUsers(users);
    } catch (err) {
      $approvalList.innerHTML = `<div class="auth-error">${err.message}</div>`;
    }
  }

  function renderPendingUsers(users) {
    $approvalList.innerHTML = '';
    if (!users.length) {
      $approvalList.innerHTML = '<p class="hint-text">No pending registrations.</p>';
      return;
    }
    users.forEach(user => {
      const row = document.createElement('div');
      row.className = 'approval-item';
      row.innerHTML = `
        <div class="approval-info">
          <strong>${escapeHtml(user.displayName || user.username)}</strong>
          <span>${escapeHtml(user.email)}</span>
          <small>Requested role: ${formatRole(user.role)}</small>
        </div>
        <div class="approval-actions">
          <select class="map-input approval-role">
            <option value="CONSUMER" ${user.role === 'CONSUMER' ? 'selected' : ''}>Consumer</option>
            <option value="PRODUCER" ${user.role === 'PRODUCER' ? 'selected' : ''}>Producer</option>
          </select>
          <button class="btn-secondary">Approve</button>
        </div>
      `;
      const select = row.querySelector('select');
      row.querySelector('button').addEventListener('click', async () => {
        await API.approveUser(user.id, select.value);
        toast(`${user.username} approved`, 'success');
        await loadPendingUsers();
      });
      $approvalList.appendChild(row);
    });
  }

  function formatRole(role) {
    return String(role || '').toLowerCase().replace(/^\w/, c => c.toUpperCase());
  }

  function escapeHtml(value) {
    return String(value || '').replace(/[&<>"']/g, ch => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    }[ch]));
  }

  function toast(message, type = 'info') {
    const container = $('toastContainer');
    const el = document.createElement('div');
    el.className = `toast ${type}`;
    const icons = { success: '✅', error: '❌', info: 'ℹ️' };
    el.innerHTML = `<span>${icons[type] || 'ℹ️'}</span><span>${message}</span>`;
    container.appendChild(el);
    setTimeout(() => { el.style.opacity = '0'; el.style.transform = 'translateX(20px)'; el.style.transition = '0.3s'; setTimeout(() => el.remove(), 300); }, 3000);
  }

  // ─── Utilities ────────────────────────────────────────────────────────────

  function timeAgo(date) {
    const s = Math.floor((Date.now() - date.getTime()) / 1000);
    if (s < 60) return `${s}s ago`;
    if (s < 3600) return `${Math.floor(s / 60)}m ago`;
    if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
    return `${Math.floor(s / 86400)}d ago`;
  }

  function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

  function emptyFeatureCollection() {
    return { type: 'FeatureCollection', features: [] };
  }

  // ─── Bootstrap ────────────────────────────────────────────────────────────

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
