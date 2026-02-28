import { useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";

interface SearchFilterProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}

export function SearchFilter({
  value,
  onChange,
  placeholder = "Search...",
  className,
}: SearchFilterProps) {
  const [local, setLocal] = useState(value);
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    setLocal(value);
  }, [value]);

  useEffect(() => {
    return () => clearTimeout(timerRef.current);
  }, []);

  function handleChange(v: string) {
    setLocal(v);
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => onChange(v), 200);
  }

  return (
    <div className={`relative ${className ?? ""}`}>
      <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <input
        type="text"
        value={local}
        onChange={(e) => handleChange(e.target.value)}
        placeholder={placeholder}
        className="h-9 w-full rounded-md border border-input bg-background pl-9 pr-3 text-sm outline-none ring-ring focus-visible:ring-2"
      />
    </div>
  );
}
