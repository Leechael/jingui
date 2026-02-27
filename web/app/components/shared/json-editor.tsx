import { useState } from "react";

interface JsonEditorProps {
  value: string;
  onChange: (value: string) => void;
  label?: string;
  rows?: number;
}

export function JsonEditor({
  value,
  onChange,
  label,
  rows = 8,
}: JsonEditorProps) {
  const [error, setError] = useState<string | null>(null);

  function handleChange(v: string) {
    onChange(v);
    try {
      JSON.parse(v);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }

  return (
    <div className="space-y-1">
      {label && (
        <label className="text-sm font-medium">{label}</label>
      )}
      <textarea
        value={value}
        onChange={(e) => handleChange(e.target.value)}
        rows={rows}
        className="w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm outline-none ring-ring focus-visible:ring-2"
        spellCheck={false}
      />
      {error && (
        <p className="text-xs text-destructive">Invalid JSON: {error}</p>
      )}
    </div>
  );
}
