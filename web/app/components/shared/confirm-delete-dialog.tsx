import { useEffect, useState } from "react";
import { Trash2 } from "lucide-react";

interface ConfirmDeleteDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: (cascade: boolean) => void;
  title: string;
  description: string;
  showCascade?: boolean;
  cascadeLabel?: string;
  isPending?: boolean;
}

export function ConfirmDeleteDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  showCascade = false,
  cascadeLabel = "Also delete dependent resources",
  isPending = false,
}: ConfirmDeleteDialogProps) {
  const [cascade, setCascade] = useState(false);

  useEffect(() => {
    if (open) setCascade(false);
  }, [open]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="fixed inset-0 bg-black/50" onClick={onClose} />
      <div className="relative z-50 w-full max-w-md rounded-lg border bg-background p-6 shadow-lg">
        <div className="flex items-center gap-3 mb-4">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-destructive/10">
            <Trash2 className="h-5 w-5 text-destructive" />
          </div>
          <div>
            <h2 className="text-lg font-semibold">{title}</h2>
            <p className="text-sm text-muted-foreground">{description}</p>
          </div>
        </div>

        {showCascade && (
          <label className="flex items-center gap-2 rounded-md border p-3 mb-4 cursor-pointer">
            <input
              type="checkbox"
              checked={cascade}
              onChange={(e) => setCascade(e.target.checked)}
              className="h-4 w-4 rounded border-input"
            />
            <span className="text-sm">{cascadeLabel}</span>
          </label>
        )}

        <div className="flex justify-end gap-2">
          <button
            onClick={onClose}
            className="rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent"
            disabled={isPending}
          >
            Cancel
          </button>
          <button
            onClick={() => onConfirm(cascade)}
            disabled={isPending}
            className="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-white hover:bg-destructive/90 disabled:opacity-50"
          >
            {isPending ? "Deleting..." : "Delete"}
          </button>
        </div>
      </div>
    </div>
  );
}
