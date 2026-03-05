// Package geojson provides GeoJSON types and helpers for constructing features.
package geojson

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

// Feature represents a single GeoJSON feature.
type Feature struct {
	Type       string         `json:"type"`
	Geometry   map[string]any `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

// FeatureCollection is a GeoJSON FeatureCollection.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

// NewFeatureCollection creates an empty FeatureCollection.
func NewFeatureCollection() *FeatureCollection {
	return &FeatureCollection{
		Type:     "FeatureCollection",
		Features: make([]Feature, 0),
	}
}

// Add appends one or more features to the collection.
func (fc *FeatureCollection) Add(features ...Feature) {
	fc.Features = append(fc.Features, features...)
}

// Marshal returns indented JSON for the collection.
func (fc *FeatureCollection) Marshal() ([]byte, error) {
	return json.MarshalIndent(fc, "", "  ")
}

// NewPointFeature creates a GeoJSON Point feature at the given lon/lat with the given properties.
func NewPointFeature(lon, lat float64, properties map[string]any) Feature {
	return Feature{
		Type: "Feature",
		Geometry: map[string]any{
			"type":        "Point",
			"coordinates": []float64{lon, lat},
		},
		Properties: properties,
	}
}

// NewPolygonFeature creates a GeoJSON Polygon feature from the given ring of [lon, lat] coordinate pairs.
// The ring should be closed (first == last) or this function will close it.
func NewPolygonFeature(ring [][]float64, properties map[string]any) Feature {
	// Close the ring if needed
	if len(ring) > 0 {
		first := ring[0]
		last := ring[len(ring)-1]
		if first[0] != last[0] || first[1] != last[1] {
			ring = append(ring, first)
		}
	}

	return Feature{
		Type: "Feature",
		Geometry: map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{ring},
		},
		Properties: properties,
	}
}

// NewLineStringFeature creates a GeoJSON LineString feature from [lon, lat] coordinate pairs.
func NewLineStringFeature(coords [][]float64, properties map[string]any) Feature {
	return Feature{
		Type: "Feature",
		Geometry: map[string]any{
			"type":        "LineString",
			"coordinates": coords,
		},
		Properties: properties,
	}
}

// LeafletVersion is the version of Leaflet.js used in the HTML template.
// Check for newer versions at https://leafletjs.com/download.html
const LeafletVersion = "1.9.4"

// leafletHTMLTemplate is a self-contained HTML page that renders GeoJSON data
// using Leaflet.js. It respects simplestyle-spec properties on features.
var leafletHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>GeoJSON Viewer</title>
  <link rel="stylesheet" href="https://unpkg.com/leaflet@` + LeafletVersion + `/dist/leaflet.css" />
  <style>
    html, body { height: 100%; margin: 0; }
    #map { height: 100%; }
  </style>
</head>
<body>
  <div id="map"></div>
  <script src="https://unpkg.com/leaflet@` + LeafletVersion + `/dist/leaflet.js"></script>
  <script>
    const geojson = GEOJSON_PLACEHOLDER;

    const map = L.map('map', { preferCanvas: true });
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors'
    }).addTo(map);

    const layer = L.geoJSON(geojson, {
      pointToLayer: (feature, latlng) => {
        const p = feature.properties || {};
        return L.circleMarker(latlng, {
          radius: p['marker-size'] === 'large' ? 10 : p['marker-size'] === 'small' ? 4 : 6,
          color: p['marker-color'] || '#7e7e7e',
          weight: 2,
          fillColor: p['marker-color'] || '#7e7e7e',
          fillOpacity: 0.8
        });
      },

      style: (feature) => {
        const p = feature.properties || {};
        const s = {};
        if (p['stroke'] != null)         s.color       = p['stroke'];
        if (p['stroke-width'] != null)   s.weight      = p['stroke-width'];
        if (p['stroke-opacity'] != null) s.opacity      = p['stroke-opacity'];
        if (p['fill'] != null)           s.fillColor   = p['fill'];
        if (p['fill-opacity'] != null)   s.fillOpacity = p['fill-opacity'];
        return s;
      },

      onEachFeature: (feature, leafletLayer) => {
        const props = feature.properties || {};
        leafletLayer.bindPopup(
          '<pre style="margin:0;white-space:pre-wrap;">' +
          escapeHtml(JSON.stringify(props, null, 2)) + '</pre>'
        );
      }
    }).addTo(map);

    const bounds = layer.getBounds();
    if (bounds.isValid()) map.fitBounds(bounds.pad(0.1));
    else map.setView([0, 0], 2);

    function escapeHtml(s) {
      return String(s)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#039;');
    }
  </script>
</body>
</html>`

// GenerateLeafletHTML returns a self-contained HTML page that renders the
// provided GeoJSON data using Leaflet.js. The GeoJSON features' simplestyle-spec
// properties (stroke, fill, marker-color, etc.) are respected in the rendering.
func GenerateLeafletHTML(geojsonData string) string {
	return strings.Replace(leafletHTMLTemplate, "GEOJSON_PLACEHOLDER", geojsonData, 1)
}

// LeafletHTMLConfig configures the HTML page generated by GenerateLeafletHTMLWithHeader.
type LeafletHTMLConfig struct {
	// Title is shown in the browser tab and as a heading in the page header.
	Title string
	// Version is displayed in the header next to the repo link (e.g. "v0.3.0").
	Version string
	// RepoURL is the URL linked from the header. If empty, no link is shown.
	RepoURL string
	// RepoName is the display text for the repo link. If empty, RepoURL is shown.
	RepoName string
	// CustomJS is arbitrary JavaScript injected at the end of the main script
	// block. It has access to `map`, `geojson`, `layer`, and `escapeHtml`.
	CustomJS string
	// DisableFeaturePopups, when true, omits the default onEachFeature handler
	// that shows a JSON properties popup on click. Useful when CustomJS provides
	// its own click behavior.
	DisableFeaturePopups bool
}

type leafletHeaderData struct {
	Title                string
	Version              string
	RepoURL              string
	RepoName             string
	GeoJSONData          template.JS
	CustomJS             template.JS
	DisableFeaturePopups bool
}

var leafletHeaderTemplate = template.Must(template.New("map").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="https://unpkg.com/leaflet@` + LeafletVersion + `/dist/leaflet.css" />
  <style>
    html, body { height: 100%; margin: 0; font-family: system-ui, -apple-system, sans-serif; }
    #header { height: 32px; display: flex; align-items: center; justify-content: space-between; padding: 0 12px; background: #fff; border-bottom: 1px solid #ddd; font-size: 14px; }
    #header h1 { font-size: 14px; font-weight: 600; margin: 0; }
    #header a { color: #666; text-decoration: none; font-size: 12px; }
    #header a:hover { color: #333; text-decoration: underline; }
    #map { height: calc(100% - 33px); }
  </style>
</head>
<body>
  <div id="header">
    <h1>{{.Title}}</h1>
    {{if .RepoURL}}<a href="{{.RepoURL}}">{{.RepoName}}{{if .Version}} v{{.Version}}{{end}}</a>{{end}}
  </div>
  <div id="map"></div>
  <script src="https://unpkg.com/leaflet@` + LeafletVersion + `/dist/leaflet.js"></script>
  <script>
    const geojson = {{.GeoJSONData}};

    const map = L.map('map', { preferCanvas: true });
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors'
    }).addTo(map);

    const layer = L.geoJSON(geojson, {
      pointToLayer: (feature, latlng) => {
        const p = feature.properties || {};
        return L.circleMarker(latlng, {
          radius: p['marker-size'] === 'large' ? 10 : p['marker-size'] === 'small' ? 4 : 6,
          color: p['marker-color'] || '#7e7e7e',
          weight: 2,
          fillColor: p['marker-color'] || '#7e7e7e',
          fillOpacity: 0.8
        });
      },

      style: (feature) => {
        const p = feature.properties || {};
        const s = {};
        if (p['stroke'] != null)         s.color       = p['stroke'];
        if (p['stroke-width'] != null)   s.weight      = p['stroke-width'];
        if (p['stroke-opacity'] != null) s.opacity      = p['stroke-opacity'];
        if (p['fill'] != null)           s.fillColor   = p['fill'];
        if (p['fill-opacity'] != null)   s.fillOpacity = p['fill-opacity'];
        return s;
      },
{{if not .DisableFeaturePopups}}
      onEachFeature: (feature, leafletLayer) => {
        const props = feature.properties || {};
        leafletLayer.bindPopup(
          '<pre style="margin:0;white-space:pre-wrap;">' +
          escapeHtml(JSON.stringify(props, null, 2)) + '</pre>'
        );
      }
{{end}}
    }).addTo(map);

    const bounds = layer.getBounds();
    if (bounds.isValid()) map.fitBounds(bounds.pad(0.1));
    else map.setView([0, 0], 2);

    function escapeHtml(s) {
      return String(s)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#039;');
    }
    {{.CustomJS}}
  </script>
</body>
</html>`))

// GenerateLeafletHTMLWithHeader returns a self-contained HTML page with a slim
// header bar showing the page title, repo link, and version. The GeoJSON
// features' simplestyle-spec properties are respected in the rendering.
func GenerateLeafletHTMLWithHeader(geojsonData string, cfg LeafletHTMLConfig) (string, error) {
	repoName := cfg.RepoName
	if repoName == "" {
		repoName = cfg.RepoURL
	}
	var buf strings.Builder
	err := leafletHeaderTemplate.Execute(&buf, leafletHeaderData{
		Title:                cfg.Title,
		Version:              cfg.Version,
		RepoURL:              cfg.RepoURL,
		RepoName:             repoName,
		GeoJSONData:          template.JS(geojsonData),
		CustomJS:             template.JS(cfg.CustomJS),
		DisableFeaturePopups: cfg.DisableFeaturePopups,
	})
	if err != nil {
		return "", fmt.Errorf("geojson: rendering HTML template: %w", err)
	}
	return buf.String(), nil
}

// GenerateGeoJSONioURL creates a geojson.io URL for viewing the provided GeoJSON data.
func GenerateGeoJSONioURL(geojsonData string) string {
	var compactData any
	compactJSON := geojsonData
	if err := json.Unmarshal([]byte(geojsonData), &compactData); err == nil {
		if compact, err := json.Marshal(compactData); err == nil {
			compactJSON = string(compact)
		}
	}

	encodedData := url.QueryEscape(compactJSON)
	return fmt.Sprintf("https://geojson.io/#data=data:application/json,%s", encodedData)
}
