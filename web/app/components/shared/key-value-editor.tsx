import { useRef } from "react";
import { Plus, X } from "lucide-react";

export interface KeyValuePair {
  id: number;
  key: string;
  value: string;
}

let globalId = 0;
export function createPair(key = "", value = ""): KeyValuePair {
  return { id: ++globalId, key, value };
}

interface KeyValueEditorProps {
  pairs: KeyValuePair[];
  onChange: (pairs: KeyValuePair[]) => void;
}

export function KeyValueEditor({ pairs, onChange }: KeyValueEditorProps) {
  function addPair() {
    onChange([...pairs, createPair()]);
  }

  function removePair(id: number) {
    onChange(pairs.filter((p) => p.id !== id));
  }

  function updatePair(id: number, field: "key" | "value", val: string) {
    onChange(
      pairs.map((p) => (p.id === id ? { ...p, [field]: val } : p)),
    );
  }

  return (
    <div className="space-y-2">
      {pairs.map((pair) => (
        <div key={pair.id} className="flex items-center gap-2">
          <input
            type="text"
            value={pair.key}
            onChange={(e) => updatePair(pair.id, "key", e.target.value)}
            placeholder="Key"
            className="h-9 flex-1 rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
          />
          <input
            type="text"
            value={pair.value}
            onChange={(e) => updatePair(pair.id, "value", e.target.value)}
            placeholder="Value"
            className="h-9 flex-1 rounded-md border border-input bg-background px-3 text-sm outline-none ring-ring focus-visible:ring-2"
          />
          <button
            type="button"
            onClick={() => removePair(pair.id)}
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
