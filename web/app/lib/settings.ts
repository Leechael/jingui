const SETTINGS_KEY = "jingui-settings";

export interface Settings {
  apiUrl: string;
  token: string;
}

export function getSettings(): Settings | null {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Settings;
    if (!parsed.apiUrl || !parsed.token) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function saveSettings(settings: Settings): void {
  localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
}

export function clearSettings(): void {
  localStorage.removeItem(SETTINGS_KEY);
}
