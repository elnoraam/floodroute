document.addEventListener('DOMContentLoaded', () => {
    // Jakarta coordinates
    const jakarta = [-6.2088, 106.8456];
    
    const map = L.map('map').setView(jakarta, 12);

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map);

    const incidentMarkers = L.layerGroup().addTo(map);
    const routeLayers = L.layerGroup().addTo(map);

    async function fetchIncidents() {
        try {
            const response = await fetch('/api/incidents');
            const incidents = await response.json();
            
            incidentMarkers.clearLayers();
            incidents.forEach(incident => {
                const marker = L.marker([incident.latitude, incident.longitude])
                    .bindPopup(`
                        <strong>${incident.type.toUpperCase()}</strong><br>
                        Severity: ${incident.severity}/5<br>
                        ${incident.description}<br>
                        <small>Expires: ${new Date(incident.expires_at).toLocaleString()}</small>
                    `);
                marker.addTo(incidentMarkers);
            });
        } catch (error) {
            console.error('Error fetching incidents:', error);
        }
    }

    fetchIncidents();
    // Refresh every minute
    setInterval(fetchIncidents, 60000);

    map.on('click', (e) => {
        const { lat, lng } = e.latlng;
        const type = prompt("Enter incident type (flood, accident, traffic, road_closure, fallen_tree):", "flood");
        if (!type) return;
        
        const description = prompt("Enter description:");
        const severity = prompt("Enter severity (1-5):", "3");

        fetch('/api/incidents', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                type,
                description,
                severity: parseInt(severity),
                latitude: lat,
                longitude: lng
            })
        }).then(res => {
            if (res.ok) {
                alert("Incident reported!");
                fetchIncidents();
            } else {
                alert("Failed to report incident.");
            }
        });
    });

    const originInput = document.getElementById('origin');
    const destinationInput = document.getElementById('destination');
    const findRouteBtn = document.getElementById('find-route');

    findRouteBtn.addEventListener('click', async () => {
        const origin = originInput.value;
        const destination = destinationInput.value;
        
        if (!origin || !destination) {
            alert('Please enter both origin and destination');
            return;
        }
        async function geocodeIfNeeded(q) {
            // If input looks like "lat,lon" return as-is
            const coordMatch = q.trim().match(/^(-?\d+(?:\.\d+)?),\s*(-?\d+(?:\.\d+)?)$/);
            if (coordMatch) {
                return { lat: parseFloat(coordMatch[1]), lon: parseFloat(coordMatch[2]) };
            }

            // Use Nominatim for simple geocoding
            try {
                const url = `https://nominatim.openstreetmap.org/search?format=json&limit=1&q=${encodeURIComponent(q)}`;
                const resp = await fetch(url, { headers: { 'Accept-Language': 'en' } });
                const results = await resp.json();
                if (results && results.length > 0) {
                    return { lat: parseFloat(results[0].lat), lon: parseFloat(results[0].lon) };
                }
                throw new Error('no geocoding result');
            } catch (err) {
                throw err;
            }
        }

        try {
            const o = await geocodeIfNeeded(origin);
            const d = await geocodeIfNeeded(destination);

            const originParam = `${o.lat},${o.lon}`;
            const destParam = `${d.lat},${d.lon}`;

            const response = await fetch(`/api/routes?origin=${encodeURIComponent(originParam)}&destination=${encodeURIComponent(destParam)}`);
            if (!response.ok) throw new Error('route request failed');
            const route = await response.json();

            routeLayers.clearLayers();

            const geometry = JSON.parse(route.geometry);
            const routeColor = route.risk_score > 0.5 ? '#e74c3c' : (route.risk_score > 0.2 ? '#f39c12' : '#2ecc71');

            const polyline = L.geoJSON(geometry, {
                style: {
                    color: routeColor,
                    weight: 5,
                    opacity: 0.7
                }
            }).addTo(routeLayers);

            map.fitBounds(polyline.getBounds());

            if (route.hazards && route.hazards.length > 0) {
                alert(`Warning: This route has a risk score of ${Math.round(route.risk_score * 100)}%. ${route.hazards.length} hazards detected.`);
            } else {
                alert(`Safe route found! Distance: ${(route.distance / 1000).toFixed(2)} km`);
            }
        } catch (error) {
            console.error('Error fetching route:', error);
            alert('Failed to find route: ' + (error.message || 'unknown'));
        }
    });
});
