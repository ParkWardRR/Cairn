import { writable } from 'svelte/store';

export type TileProviderType = 'raster' | 'pmtiles';

export interface TileProvider {
  id: string;
  name: string;
  group: string;
  type: TileProviderType;
  url: string;
  attribution: string;
  maxZoom: number;
  dark: boolean;
}

export const TILE_PROVIDERS: TileProvider[] = [
  {
    id: 'carto-dark',
    name: 'CARTO Dark Matter',
    group: 'Dark',
    type: 'raster',
    url: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    attribution: '&copy; <a href="https://openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
    maxZoom: 20,
    dark: true,
  },
  {
    id: 'carto-dark-nolabels',
    name: 'CARTO Dark (No Labels)',
    group: 'Dark',
    type: 'raster',
    url: 'https://{s}.basemaps.cartocdn.com/dark_nolabels/{z}/{x}/{y}{r}.png',
    attribution: '&copy; <a href="https://openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
    maxZoom: 20,
    dark: true,
  },
  {
    id: 'carto-voyager',
    name: 'CARTO Voyager',
    group: 'Light',
    type: 'raster',
    url: 'https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png',
    attribution: '&copy; <a href="https://openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
    maxZoom: 20,
    dark: false,
  },
  {
    id: 'carto-positron',
    name: 'CARTO Positron',
    group: 'Light',
    type: 'raster',
    url: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
    attribution: '&copy; <a href="https://openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
    maxZoom: 20,
    dark: false,
  },
  {
    id: 'osm',
    name: 'OpenStreetMap',
    group: 'Standard',
    type: 'raster',
    url: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
    attribution: '&copy; <a href="https://openstreetmap.org/copyright">OpenStreetMap</a>',
    maxZoom: 19,
    dark: false,
  },
  {
    id: 'opentopomap',
    name: 'OpenTopoMap',
    group: 'Standard',
    type: 'raster',
    url: 'https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png',
    attribution: '&copy; <a href="https://openstreetmap.org/copyright">OSM</a> &copy; <a href="https://opentopomap.org">OpenTopoMap</a>',
    maxZoom: 17,
    dark: false,
  },
  {
    id: 'esri-satellite',
    name: 'Esri Satellite',
    group: 'Satellite',
    type: 'raster',
    url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
    attribution: '&copy; Esri, Maxar, Earthstar Geographics',
    maxZoom: 19,
    dark: true,
  },
];

export interface MapSettings {
  providerId: string;
  customTileUrl: string;
  customAttribution: string;
  pmtilesUrl: string;
}

const STORAGE_KEY = 'cairn-map-settings';

const defaults: MapSettings = {
  providerId: 'carto-dark',
  customTileUrl: '',
  customAttribution: '',
  pmtilesUrl: '',
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
    setCustomTileUrl(url: string, attribution: string) {
      store.update((s) => ({ ...s, customTileUrl: url, customAttribution: attribution, providerId: 'custom' }));
    },
    setPmtilesUrl(url: string) {
      store.update((s) => ({ ...s, pmtilesUrl: url, providerId: 'pmtiles' }));
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
