import { PathLayer, ScatterplotLayer, ArcLayer } from '@deck.gl/layers';

export function tripRouteLayers(routeData: any, opts: { colorBySpeed?: boolean } = {}) {
  if (!routeData?.features?.length) return [];

  const feature = routeData.features[0];
  const coords = feature.geometry?.coordinates || [];
  const speeds = feature.properties?.speed || [];

  if (coords.length < 2) return [];

  const layers: any[] = [];

  if (opts.colorBySpeed && speeds.length > 0) {
    const maxSpeed = Math.max(...speeds.filter((s: number) => s > 0), 60);
    const segments: { path: [number, number][]; color: [number, number, number, number] }[] = [];

    for (let i = 1; i < coords.length; i++) {
      const ratio = Math.min((speeds[i] || 0) / maxSpeed, 1);
      segments.push({
        path: [[coords[i-1][0], coords[i-1][1]], [coords[i][0], coords[i][1]]],
        color: speedToColor(ratio),
      });
    }

    layers.push(new PathLayer({
      id: 'trip-route-speed',
      data: segments,
      getPath: (d: any) => d.path,
      getColor: (d: any) => d.color,
      getWidth: 4,
      widthUnits: 'pixels',
      widthMinPixels: 2,
      capRounded: true,
      jointRounded: true,
    }));
  } else {
    layers.push(new PathLayer({
      id: 'trip-route',
      data: [{ path: coords.map((c: number[]) => [c[0], c[1]]) }],
      getPath: (d: any) => d.path,
      getColor: [255, 55, 95, 200],
      getWidth: 4,
      widthUnits: 'pixels',
      widthMinPixels: 2,
      capRounded: true,
      jointRounded: true,
    }));
  }

  const start = coords[0];
  const end = coords[coords.length - 1];
  layers.push(new ScatterplotLayer({
    id: 'trip-endpoints',
    data: [
      { position: [start[0], start[1]], color: [48, 209, 88], label: 'start' },
      { position: [end[0], end[1]], color: [255, 55, 95], label: 'end' },
    ],
    getPosition: (d: any) => d.position,
    getFillColor: (d: any) => d.color,
    getRadius: 7,
    radiusUnits: 'pixels',
    radiusMinPixels: 5,
    stroked: true,
    getLineColor: [255, 255, 255],
    getLineWidth: 2,
    lineWidthUnits: 'pixels',
  }));

  return layers;
}

export function tripEndpointLayers(trips: any[]) {
  const valid = trips.filter(t => t.start_lat && t.start_lon && t.end_lat && t.end_lon);

  return [
    new ArcLayer({
      id: 'trip-arcs',
      data: valid,
      getSourcePosition: (d: any) => [d.start_lon, d.start_lat],
      getTargetPosition: (d: any) => [d.end_lon, d.end_lat],
      getSourceColor: [48, 209, 88, 120],
      getTargetColor: [255, 55, 95, 120],
      getWidth: 1.5,
      greatCircle: true,
    }),
    new ScatterplotLayer({
      id: 'trip-starts',
      data: valid,
      getPosition: (d: any) => [d.start_lon, d.start_lat],
      getFillColor: [48, 209, 88, 180],
      getRadius: 5,
      radiusUnits: 'pixels',
      radiusMinPixels: 3,
    }),
    new ScatterplotLayer({
      id: 'trip-ends',
      data: valid,
      getPosition: (d: any) => [d.end_lon, d.end_lat],
      getFillColor: [255, 55, 95, 180],
      getRadius: 5,
      radiusUnits: 'pixels',
      radiusMinPixels: 3,
    }),
  ];
}

export function placeLayers(places: any[]) {
  const valid = places.filter(p => p.lat && p.lon);

  return [
    new ScatterplotLayer({
      id: 'place-radius',
      data: valid,
      getPosition: (d: any) => [d.lon, d.lat],
      getFillColor: [94, 92, 230, 30],
      getLineColor: [94, 92, 230, 150],
      getRadius: (d: any) => d.radius_m || 100,
      radiusUnits: 'meters',
      stroked: true,
      getLineWidth: 1.5,
      lineWidthUnits: 'pixels',
    }),
    new ScatterplotLayer({
      id: 'place-centers',
      data: valid,
      getPosition: (d: any) => [d.lon, d.lat],
      getFillColor: [48, 209, 88, 200],
      getRadius: 6,
      radiusUnits: 'pixels',
      radiusMinPixels: 4,
    }),
  ];
}

function speedToColor(ratio: number): [number, number, number, number] {
  if (ratio < 0.5) {
    const t = ratio * 2;
    return [
      Math.round(48 + (255 - 48) * t),
      Math.round(209 + (214 - 209) * t),
      Math.round(88 + (10 - 88) * t),
      220,
    ];
  }
  const t = (ratio - 0.5) * 2;
  return [
    Math.round(255),
    Math.round(214 + (55 - 214) * t),
    Math.round(10 + (95 - 10) * t),
    220,
  ];
}
