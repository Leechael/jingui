import { useEffect, useState, useSyncExternalStore } from "react";
import { CheckCircle, XCircle } from "lucide-react";
import { subscribe, getToasts } from "~/lib/toast";

export function Toaster() {
  const toasts = useSyncExternalStore(subscribe, getToasts, getToasts);

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className="flex items-center gap-2 rounded-lg border bg-background px-4 py-3 shadow-lg animate-in slide-in-from-bottom-2"
        >
          {toast.type === "success" ? (
            <CheckCircle className="h-4 w-4 text-green-600" />
          ) : (
            <XCircle className="h-4 w-4 text-destructive" />
          )}
          <span className="text-sm">{toast.message}</span>
        </div>
      ))}
    </div>
  );
}
