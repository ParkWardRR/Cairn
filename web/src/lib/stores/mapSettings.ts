import { writable } from 'svelte/store';

export type MapEngine = '2d' | '3d' | 'globe';

export interface TileProvider {
  id: string;
  name: string;
  group: string;
  dark: boolean;
  style: () => any;
}

function rasterStyle(tiles: string[], attribution: string, maxZoom = 20): any {
  return {
    version: 8,
    sources: {
      'raster-tiles': {
        type: 'raster',
        tiles,
        tileSize: 256,
        attribution,
        maxzoom: maxZoom,
      },
    },
    layers: [{ id: 'raster-layer', type: 'raster', source: 'raster-tiles' }],
  };
}

export const TILE_PROVIDERS: TileProvider[] = [
  {
    id: 'ofm-liberty',
    name: 'Liberty',
    group: 'Vector',
    dark: false,
    style: () => 'https://tiles.openfreemap.org/styles/liberty',
  },
  {
    id: 'ofm-bright',
    name: 'Bright',
    group: 'Vector',
    dark: false,
    style: () => 'https://tiles.openfreemap.org/styles/bright',
  },
  {
    id: 'ofm-positron',
    name: 'Positron',
    group: 'Vector',
    dark: false,
    style: () => 'https://tiles.openfreemap.org/styles/positron',
  },
  {
    id: 'ofm-dark',
    name: 'Dark Matter',
    group: 'Vector',
    dark: true,
    style: () => 'https://tiles.openfreemap.org/styles/dark_matter',
  },
  {
    id: 'osm',
    name: 'OpenStreetMap',
    group: 'Raster',
    dark: false,
    style: () => rasterStyle(
      ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
      '&copy; <a href="https://openstreetmap.org/copyright">OpenStreetMap</a>',
      19,
    ),
  },
  {
    id: 'opentopomap',
    name: 'OpenTopoMap',
    group: 'Raster',
    dark: false,
    style: () => rasterStyle(
      ['https://a.tile.opentopomap.org/{z}/{x}/{y}.png',
       'https://b.tile.opentopomap.org/{z}/{x}/{y}.png',
       'https://c.tile.opentopomap.org/{z}/{x}/{y}.png'],
      '&copy; <a href="https://openstreetmap.org/copyright">OSM</a> &copy; <a href="https://opentopomap.org">OpenTopoMap</a>',
      17,
    ),
  },
  {
    id: 'esri-satellite',
    name: 'Esri Satellite',
    group: 'Satellite',
    dark: true,
    style: () => rasterStyle(
      ['https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'],
      '&copy; Esri, Maxar, Earthstar Geographics',
      19,
    ),
  },
];

export interface MapSettings {
  providerId: string;
  engine: MapEngine;
  customTileUrl: string;
  customAttribution: string;
  pmtilesUrl: string;
  terrain: boolean;
  pitch: number;
  bearing: number;
}

const STORAGE_KEY = 'cairn-map-settings';

const defaults: MapSettings = {
  providerId: 'ofm-liberty',
  engine: '2d',
  customTileUrl: '',
  customAttribution: '',
  pmtilesUrl: '',
  terrain: false,
  pitch: 0,
  bearing: 0,
};

function loadSettings(): MapSettings {
  if (typeof window === 'undefined') return defaults;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaults;
    return { ...defaults, ...JSON.parse(raw) };
  } catch {
    return defaults;
  }
}

function createMapSettings() {
  const store = writable<MapSettings>(loadSettings());

  store.subscribe((v) => {
    if (typeof window === 'undefined') return;
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(v)); } catch {}
  });

  return {
    subscribe: store.subscribe,
    set: store.set,
    update: store.update,
    setProvider(id: string) {
      store.update((s) => ({ ...s, providerId: id }));
    },
    setEngine(engine: MapEngine) {
      store.update((s) => ({ ...s, engine }));
    },
    setCustomTileUrl(url: string, attribution: string) {
      store.update((s) => ({ ...s, customTileUrl: url, customAttribution: attribution, providerId: 'custom' }));
    },
    setPmtilesUrl(url: string) {
      store.update((s) => ({ ...s, pmtilesUrl: url, providerId: 'pmtiles' }));
    },
    setTerrain(enabled: boolean) {
      store.update((s) => ({ ...s, terrain: enabled }));
    },
    reset() {
      store.set(defaults);
    },
  };
}

export const mapSettings = createMapSettings();

export function getActiveProvider(settings: MapSettings): TileProvider | null {
  if (settings.providerId === 'custom' || settings.providerId === 'pmtiles') return null;
  return TILE_PROVIDERS.find((p) => p.id === settings.providerId) ?? TILE_PROVIDERS[0];
}

export function getMapStyle(settings: MapSettings): any {
  if (settings.providerId === 'custom' && settings.customTileUrl) {
    return rasterStyle(
      [settings.customTileUrl],
      settings.customAttribution || 'Custom tiles',
    );
  }
  if (settings.providerId === 'pmtiles' && settings.pmtilesUrl) {
    return {
      version: 8,
      sources: {
        'pmtiles-source': {
          type: 'vector',
          url: `pmtiles://${settings.pmtilesUrl}`,
        },
      },
      layers: [],
    };
  }
  const provider = getActiveProvider(settings);
  if (provider) return provider.style();
  return TILE_PROVIDERS[0].style();
}
