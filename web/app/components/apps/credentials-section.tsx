import { useState } from "react";
import { KeyRound, ExternalLink, Copy, Check } from "lucide-react";
import { usePutCredentials, useStartDeviceFlow } from "~/lib/mutations";
import { getClient } from "~/lib/api-client";
import {
  KeyValueEditor,
  createPair,
  type KeyValuePair,
} from "~/components/shared/key-value-editor";

interface CredentialsSectionProps {
  appId: string;
}

export function CredentialsSection({ appId }: CredentialsSectionProps) {
  const [tab, setTab] = useState<"direct" | "oauth" | "device">("direct");

  return (
    <div className="space-y-4">
      <h3 className="text-lg font-semibold">Store Credentials</h3>
      <div className="flex gap-1 rounded-lg bg-muted p-1">
        {(["direct", "oauth", "device"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
              tab === t
                ? "bg-background shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            {t === "direct" ? "Direct" : t === "oauth" ? "OAuth Gateway" : "Device Flow"}
          </button>
        ))}
      </div>

      {tab === "direct" && <DirectCredentials appId={appId} />}
      {tab === "oauth" && <OAuthGateway appId={appId} />}
      {tab === "device" && <DeviceFlow appId={appId} />}
    </div>
  );
}

function DirectCredentials({ appId }: { appId: string }) {
  const putCreds = usePutCredentials(appId);
  const [item, setItem] = useState("");
  const [pairs, setPairs] = useState<KeyValuePair[]>([
    createPair(),
  ]);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const secrets: Record<string, string> = {};
    for (const p of pairs) {
      if (p.key.trim()) secrets[p.key.trim()] = p.value;
    }
    if (!item.trim() || Object.keys(secrets).length === 0) return;
    putCreds.mutate(
      { item: item.trim(), secrets },
      {
        onSuccess: () => {
          setItem("");
          setPairs([createPair()]);
        },
      },
    );
  }

  return (
    <form onSubmit={handleSubmit} className="rounded-lg border p-4 space-y-4">
      <div className="space-y-2">
        <label className="text-sm font-medium">Item Name</label>
        <input
          type="text"
          value={item}
          onChange={(e) => setItem(e.target.value)}
          required
          placeholder="alice@gmail.com"
          className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">Secrets</label>
        <KeyValueEditor pairs={pairs} onChange={setPairs} />
      </div>

      {putCreds.error && (
        <p className="text-sm text-destructive">{putCreds.error.message}</p>
      )}

      <button
        type="submit"
        disabled={putCreds.isPending}
        className="flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
      >
        <KeyRound className="h-4 w-4" />
        {putCreds.isPending ? "Storing..." : "Store Credentials"}
      </button>

      {putCreds.isSuccess && (
        <p className="text-sm text-green-600">Credentials stored successfully.</p>
      )}
    </form>
  );
}

function OAuthGateway({ appId }: { appId: string }) {
  function handleOAuth() {
    const url = getClient().getOAuthGatewayUrl(appId);
    window.open(url, "_blank");
  }

  return (
    <div className="rounded-lg border p-4 space-y-3">
      <p className="text-sm text-muted-foreground">
        Opens the OAuth provider login in a new window. After authorization, credentials are stored automatically.
      </p>
      <button
        onClick={handleOAuth}
        className="flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
      >
        <ExternalLink className="h-4 w-4" />
        Start OAuth Flow
      </button>
    </div>
  );
}

function DeviceFlow({ appId }: { appId: string }) {
  const deviceFlow = useStartDeviceFlow(appId);
  const [copied, setCopied] = useState(false);

  async function handleCopy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard write can fail in insecure contexts
    }
  }

  return (
    <div className="rounded-lg border p-4 space-y-3">
      {!deviceFlow.data ? (
        <>
          <p className="text-sm text-muted-foreground">
            Start a device authorization flow. You'll receive a code to enter on the provider's website.
          </p>
          <button
            onClick={() => deviceFlow.mutate()}
            disabled={deviceFlow.isPending}
            className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            {deviceFlow.isPending ? "Starting..." : "Start Device Flow"}
          </button>
          {deviceFlow.error && (
            <p className="text-sm text-destructive">{deviceFlow.error.message}</p>
          )}
        </>
      ) : (
        <div className="space-y-3">
          <div className="rounded-md bg-muted p-4 text-center">
            <p className="text-sm text-muted-foreground mb-2">Enter this code:</p>
            <div className="flex items-center justify-center gap-2">
              <code className="text-2xl font-bold tracking-widest">
                {deviceFlow.data.user_code}
              </code>
              <button
                onClick={() => handleCopy(deviceFlow.data!.user_code)}
                className="rounded-md p-1 hover:bg-accent"
              >
                {copied ? (
                  <Check className="h-4 w-4 text-green-600" />
                ) : (
                  <Copy className="h-4 w-4 text-muted-foreground" />
                )}
              </button>
            </div>
          </div>
          <a
            href={deviceFlow.data.verification_url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center justify-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            <ExternalLink className="h-4 w-4" />
            Open Verification URL
          </a>
          <p className="text-xs text-muted-foreground text-center">
            Expires in {Math.floor(deviceFlow.data.expires_in / 60)} minutes
          </p>
        </div>
      )}
    </div>
  );
}
