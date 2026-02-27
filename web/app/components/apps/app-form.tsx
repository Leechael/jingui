import { useState } from "react";
import { JsonEditor } from "~/components/shared/json-editor";
import type { AppRequest } from "~/lib/types";

interface AppFormProps {
  defaultValues?: Partial<AppRequest>;
  onSubmit: (data: AppRequest) => void;
  isPending: boolean;
  error?: string;
  isEdit?: boolean;
}

export function AppForm({
  defaultValues,
  onSubmit,
  isPending,
  error,
  isEdit,
}: AppFormProps) {
  const [vault, setVault] = useState(defaultValues?.vault ?? "");
  const [name, setName] = useState(defaultValues?.name ?? "");
  const [serviceType, setServiceType] = useState(
    defaultValues?.service_type ?? "",
  );
  const [requiredScopes, setRequiredScopes] = useState(
    defaultValues?.required_scopes ?? "",
  );
  const [credentialsJson, setCredentialsJson] = useState(
    defaultValues?.credentials_json
      ? JSON.stringify(defaultValues.credentials_json, null, 2)
      : '{\n  "web": {}\n}',
  );

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(credentialsJson);
    } catch {
      return;
    }
    onSubmit({
      vault,
      name,
      service_type: serviceType,
      required_scopes: requiredScopes || undefined,
      credentials_json: parsed,
    });
  }

  return (
    <form onSubmit={handleSubmit} className="max-w-lg space-y-4 rounded-lg border p-6">
      <div className="space-y-2">
        <label className="text-sm font-medium">Vault ID</label>
        <input
          type="text"
          value={vault}
          onChange={(e) => setVault(e.target.value)}
          required
          disabled={isEdit}
          placeholder="my-gmail"
          className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2 disabled:opacity-50"
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">Name</label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          placeholder="My Gmail App"
          className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">Service Type</label>
        <input
          type="text"
          value={serviceType}
          onChange={(e) => setServiceType(e.target.value)}
          required
          placeholder="google-gmail"
          className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
        />
      </div>

      <div className="space-y-2">
        <label className="text-sm font-medium">Required Scopes (optional)</label>
        <input
          type="text"
          value={requiredScopes}
          onChange={(e) => setRequiredScopes(e.target.value)}
          placeholder="https://www.googleapis.com/auth/gmail.readonly"
          className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
        />
      </div>

      <JsonEditor
        value={credentialsJson}
        onChange={setCredentialsJson}
        label="Credentials JSON"
      />

      {error && (
        <p className="text-sm text-destructive">{error}</p>
      )}

      <button
        type="submit"
        disabled={isPending}
        className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
      >
        {isPending ? "Saving..." : isEdit ? "Update App" : "Create App"}
      </button>
    </form>
  );
}
