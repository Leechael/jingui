import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { AppShell } from "~/components/layout/app-shell";
import { getSettings, saveSettings, clearSettings } from "~/lib/settings";
import { clearClientCache } from "~/lib/api-client";
import { CheckCircle, XCircle, Loader2 } from "lucide-react";

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
});

function SettingsPage() {
  const existing = getSettings();
  const [apiUrl, setApiUrl] = useState(existing?.apiUrl ?? "");
  const [token, setToken] = useState(existing?.token ?? "");
  const [testResult, setTestResult] = useState<
    "idle" | "loading" | "success" | "error"
  >("idle");
  const [testError, setTestError] = useState("");
  const [saved, setSaved] = useState(false);

  async function handleTestConnection() {
    if (!apiUrl) return;
    setTestResult("loading");
    try {
      const url = apiUrl.replace(/\/+$/, "");
      const res = await fetch(`${url}/`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (res.ok) {
        setTestResult("success");
      } else {
        setTestResult("error");
        setTestError(`Server returned ${res.status}`);
      }
    } catch (e) {
      setTestResult("error");
      setTestError((e as Error).message);
    }
  }

  function handleSave() {
    saveSettings({ apiUrl: apiUrl.replace(/\/+$/, ""), token });
    clearClientCache();
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  function handleClear() {
    clearSettings();
    clearClientCache();
    setApiUrl("");
    setToken("");
    setTestResult("idle");
  }

  return (
    <AppShell>
      <div className="space-y-6">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Settings</h2>
          <p className="text-sm text-muted-foreground">
            Configure your Jingui server connection.
          </p>
        </div>

        <div className="max-w-lg space-y-4 rounded-lg border p-6">
          <div className="space-y-2">
            <label className="text-sm font-medium">API URL</label>
            <input
              type="url"
              value={apiUrl}
              onChange={(e) => {
                setApiUrl(e.target.value);
                setTestResult("idle");
              }}
              placeholder="https://your-jingui-server.example.com"
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
            />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">Bearer Token</label>
            <input
              type="password"
              value={token}
              onChange={(e) => {
                setToken(e.target.value);
                setTestResult("idle");
              }}
              placeholder="Your admin token"
              className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
            />
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={handleTestConnection}
              disabled={!apiUrl || testResult === "loading"}
              className="rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent disabled:opacity-50"
            >
              {testResult === "loading" ? (
                <span className="flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" /> Testing...
                </span>
              ) : (
                "Test Connection"
              )}
            </button>
            {testResult === "success" && (
              <span className="flex items-center gap-1 text-sm text-green-600">
                <CheckCircle className="h-4 w-4" /> Connected
              </span>
            )}
            {testResult === "error" && (
              <span className="flex items-center gap-1 text-sm text-destructive">
                <XCircle className="h-4 w-4" /> {testError}
              </span>
            )}
          </div>

          <div className="flex gap-2 border-t pt-4">
            <button
              onClick={handleSave}
              disabled={!apiUrl || !token}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {saved ? "Saved!" : "Save"}
            </button>
            <button
              onClick={handleClear}
              className="rounded-md border px-4 py-2 text-sm font-medium text-destructive hover:bg-accent"
            >
              Clear
            </button>
          </div>
        </div>
      </div>
    </AppShell>
  );
}
