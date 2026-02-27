import { Plus, X } from "lucide-react";

export interface KeyValuePair {
  key: string;
  value: string;
}

interface KeyValueEditorProps {
  pairs: KeyValuePair[];
  onChange: (pairs: KeyValuePair[]) => void;
}

export function KeyValueEditor({ pairs, onChange }: KeyValueEditorProps) {
  function addPair() {
    onChange([...pairs, { key: "", value: "" }]);
  }

  function removePair(index: number) {
    onChange(pairs.filter((_, i) => i !== index));
  }

  function updatePair(index: number, field: "key" | "value", val: string) {
    const updated = pairs.map((p, i) =>
      i === index ? { ...p, [field]: val } : p,
    );
    onChange(updated);
  }

  return (
    <div className="space-y-2">
      {pairs.map((pair, i) => (
        <div key={i} className="flex items-center gap-2">
          <input
            type="text"
            value={pair.key}
            onChange={(e) => updatePair(i, "key", e.target.value)}
            placeholder="Key"
            className="h-9 flex-1 rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
          />
          <input
            type="text"
            value={pair.value}
            onChange={(e) => updatePair(i, "value", e.target.value)}
            placeholder="Value"
            className="h-9 flex-1 rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
          />
          <button
            type="button"
            onClick={() => removePair(i)}
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border hover:bg-accent"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={addPair}
        className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <Plus className="h-4 w-4" />
        Add pair
      </button>
    </div>
  );
}
