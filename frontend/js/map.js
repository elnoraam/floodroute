// ─── FloodRoute Map Module ────────────────────────────────────────────────────

const MapManager = (() => {
  let map, routeLayerGroup, floodLayerGroup, incidentLayerGroup;
  let originMarker = null, destMarker = null;

  // Custom icon factory
  function makeIcon(emoji, color = '#06b6d4', size = 32) {
    const svg = `
      <svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size + 6}" viewBox="0 0 32 38">
        <circle cx="16" cy="14" r="12" fill="${color}" fill-opacity="0.9" stroke="white" stroke-width="1.5"/>
        <polygon points="10,24 22,24 16,34" fill="${color}" fill-opacity="0.9"/>
        <text x="16" y="19" text-anchor="middle" font-size="13">${emoji}</text>
      </svg>`;
    return L.divIcon({
      html: svg,
      className: '',
      iconSize: [size, size + 6],
      iconAnchor: [size / 2, size + 6],
      popupAnchor: [0, -(size + 6)]
    });
  }

  const INCIDENT_ICONS = {
    FLOOD:        makeIcon('🌊', '#3b82f6'),
    ACCIDENT:     makeIcon('🚨', '#ef4444'),
    CONGESTION:   makeIcon('🚦', '#f59e0b'),
    ROAD_CLOSURE: makeIcon('🚧', '#ef4444'),
    FALLEN_TREE:  makeIcon('🌲', '#22c55e'),
    OTHER:        makeIcon('⚠️', '#8b5cf6'),
  };

  const ORIGIN_ICON = makeIcon('📍', '#22c55e', 34);
  const DEST_ICON   = makeIcon('🏁', '#ef4444', 34);

  function safetyColor(score) {
    if (score >= 0.8) return '#22c55e';
    if (score >= 0.5) return '#f59e0b';
    return '#ef4444';
  }

  function riskColor(level) {
    const colors = { 1: '#22c55e', 2: '#84cc16', 3: '#f59e0b', 4: '#f97316', 5: '#ef4444' };
    return colors[level] || '#f59e0b';
  }

  function init(center = [-6.9175, 107.6191], zoom = 13) {
    map = L.map('map', {
      center,
      zoom,
      zoomControl: false,
      attributionControl: true
    });

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© OpenStreetMap contributors',
      maxZoom: 19
    }).addTo(map);

    routeLayerGroup    = L.layerGroup().addTo(map);
    floodLayerGroup    = L.layerGroup().addTo(map);
    incidentLayerGroup = L.layerGroup().addTo(map);

    return map;
  }

  // ─── Flood Zones ─────────────────────────────────────────────────────────

  function renderFloodZones(geojson) {
    floodLayerGroup.clearLayers();
    if (!geojson || !geojson.features) return;

    L.geoJSON(geojson, {
      style(feature) {
        const risk = feature.properties.riskLevel || 3;
        const col = riskColor(risk);
        return {
          fillColor: col,
          fillOpacity: 0.22,
          color: col,
          weight: 1.5,
          opacity: 0.7,
          dashArray: risk >= 4 ? null : '6 4'
        };
      },
      onEachFeature(feature, layer) {
        const p = feature.properties;
        layer.bindPopup(`
          <div class="popup-title">⚠ Flood Zone</div>
          <div class="popup-body">
            <strong>${p.name || 'Unnamed zone'}</strong><br/>
            Risk Level: ${'★'.repeat(p.riskLevel)}${'☆'.repeat(5 - p.riskLevel)}<br/>
            Rainfall: ${p.rainfallMm ? p.rainfallMm + ' mm' : 'N/A'}<br/>
            Source: ${p.source}
          </div>
        `);
        layer.on('mouseover', () => layer.setStyle({ fillOpacity: 0.4 }));
        layer.on('mouseout', () => layer.setStyle({ fillOpacity: 0.22 }));
      }
    }).addTo(floodLayerGroup);
  }

  // ─── Incidents ────────────────────────────────────────────────────────────

  function renderIncidents(geojson) {
    incidentLayerGroup.clearLayers();
    if (!geojson || !geojson.features) return;

    geojson.features.forEach(f => {
      const p = f.properties;
      const [lon, lat] = f.geometry.coordinates;
      const icon = INCIDENT_ICONS[p.type] || INCIDENT_ICONS.OTHER;

      const marker = L.marker([lat, lon], { icon });
      const when = p.reportedAt ? new Date(p.reportedAt).toLocaleString('id-ID') : '';
      marker.bindPopup(`
        <div class="popup-title">${incidentEmoji(p.type)} ${formatIncidentType(p.type)}</div>
        <div class="popup-body">
          ${p.title ? `<strong>${p.title}</strong><br/>` : ''}
          ${p.description || ''}<br/>
          <span style="color:var(--text-muted)">Dilaporkan: ${when}</span><br/>
          👍 ${p.upvotes || 0} upvotes
          ${p.isVerified ? ' ✅ Verified' : ''}
        </div>
      `);
      marker.addTo(incidentLayerGroup);
    });
  }

  // ─── Routes ───────────────────────────────────────────────────────────────

  function renderRoutes(routeData, selectedIdx = 0) {
    routeLayerGroup.clearLayers();
    if (!routeData) return;

    const allRoutes = [routeData.recommended, ...(routeData.alternatives || [])];

    allRoutes.forEach((route, i) => {
      if (!route.geometry || !route.geometry.coordinates) return;

      const color = safetyColor(route.safetyScore);
      const isSelected = i === selectedIdx;

      // Shadow line for depth
      L.geoJSON(route.geometry, {
        style: { color: '#000', weight: isSelected ? 9 : 6, opacity: 0.3 }
      }).addTo(routeLayerGroup);

      // Main route line
      const line = L.geoJSON(route.geometry, {
        style: {
          color,
          weight: isSelected ? 5 : 3,
          opacity: isSelected ? 1 : 0.45,
          dashArray: isSelected ? null : '10 6',
          lineCap: 'round',
          lineJoin: 'round'
        }
      }).addTo(routeLayerGroup);

      // Animated flow for selected route
      if (isSelected) {
        const flowLine = L.geoJSON(route.geometry, {
          style: {
            color: 'white',
            weight: 2,
            opacity: 0.6,
            dashArray: '8 16',
            dashOffset: '0'
          }
        }).addTo(routeLayerGroup);

        // Animate dash offset
        let offset = 0;
        const animate = () => {
          offset = (offset + 1) % 24;
          flowLine.setStyle({ dashOffset: String(offset) });
          requestAnimationFrame(animate);
        };
        requestAnimationFrame(animate);
      }

      // Hazard markers along route
      if (isSelected && route.hazards) {
        route.hazards.forEach(h => {
          const hIcon = makeIcon(hazardEmoji(h.type), '#ef4444', 26);
          const m = L.marker([h.lat, h.lon], { icon: hIcon });
          m.bindPopup(`
            <div class="popup-title">${hazardEmoji(h.type)} ${h.type.replace(/_/g,' ')}</div>
            <div class="popup-body">
              Severity: ${'★'.repeat(h.severity)}${'☆'.repeat(5 - h.severity)}<br/>
              ${h.description || ''}
            </div>
          `);
          m.addTo(routeLayerGroup);
        });
      }
    });
  }

  // ─── Origin / Destination ─────────────────────────────────────────────────

  function setOrigin(lat, lon) {
    if (originMarker) map.removeLayer(originMarker);
    originMarker = L.marker([lat, lon], { icon: ORIGIN_ICON, zIndexOffset: 1000 }).addTo(map);
    originMarker.bindPopup('<div class="popup-title">📍 Origin</div>');
  }

  function setDest(lat, lon) {
    if (destMarker) map.removeLayer(destMarker);
    destMarker = L.marker([lat, lon], { icon: DEST_ICON, zIndexOffset: 1000 }).addTo(map);
    destMarker.bindPopup('<div class="popup-title">🏁 Destination</div>');
  }

  function clearMarkers() {
    if (originMarker) { map.removeLayer(originMarker); originMarker = null; }
    if (destMarker) { map.removeLayer(destMarker); destMarker = null; }
    routeLayerGroup.clearLayers();
  }

  function fitToRoutes(routeData) {
    if (!routeData?.recommended?.geometry?.coordinates?.length) return;
    const coords = routeData.recommended.geometry.coordinates.map(c => [c[1], c[0]]);
    if (coords.length) map.fitBounds(L.latLngBounds(coords), { padding: [40, 40] });
  }

  function fitToAll() {
    const bounds = [];
    floodLayerGroup.eachLayer(l => {
      if (l.getBounds) bounds.push(...Object.values(l.getBounds()));
    });
    incidentLayerGroup.eachLayer(l => {
      if (l.getLatLng) bounds.push(l.getLatLng());
    });
    if (bounds.length) map.fitBounds(L.latLngBounds(bounds), { padding: [40, 40] });
  }

  // ─── Layer visibility ─────────────────────────────────────────────────────

  function setLayerVisible(name, visible) {
    const groups = { floodZones: floodLayerGroup, incidents: incidentLayerGroup, routes: routeLayerGroup };
    const g = groups[name];
    if (!g) return;
    if (visible) map.addLayer(g);
    else map.removeLayer(g);
  }

  // ─── Helpers ─────────────────────────────────────────────────────────────

  function incidentEmoji(type) {
    const m = { FLOOD:'🌊', ACCIDENT:'🚨', CONGESTION:'🚦', ROAD_CLOSURE:'🚧', FALLEN_TREE:'🌲', OTHER:'⚠️' };
    return m[type] || '⚠️';
  }

  function formatIncidentType(type) {
    return (type || '').replace(/_/g, ' ').toLowerCase().replace(/\b\w/g, c => c.toUpperCase());
  }

  function hazardEmoji(type) {
    const m = { FLOOD:'🌊', FLOOD_ZONE:'💧', ACCIDENT:'🚨', CONGESTION:'🚦', ROAD_CLOSURE:'🚧', FALLEN_TREE:'🌲' };
    return m[type] || '⚠️';
  }

  return {
    init,
    getMap() { return map; },
    renderFloodZones,
    renderIncidents,
    renderRoutes,
    setOrigin,
    setDest,
    clearMarkers,
    fitToRoutes,
    fitToAll,
    setLayerVisible,
    safetyColor,
    incidentEmoji,
    formatIncidentType
  };
})();