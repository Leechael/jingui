import { Link, useRouterState } from "@tanstack/react-router";
import {
  AppWindow,
  Server,
  KeyRound,
  Settings,
} from "lucide-react";
import { cn } from "~/lib/utils";

const navItems = [
  { to: "/apps", label: "Apps", icon: AppWindow },
  { to: "/instances", label: "Instances", icon: Server },
  { to: "/secrets", label: "Secrets", icon: KeyRound },
  { to: "/settings", label: "Settings", icon: Settings },
] as const;

export function SidebarNav() {
  const routerState = useRouterState();
  const pathname = routerState.location.pathname;

  return (
    <nav className="flex flex-col gap-1 p-3">
      <div className="mb-4 px-3 py-2">
        <h1 className="text-lg font-bold tracking-tight">Jingui</h1>
        <p className="text-xs text-muted-foreground">Admin Panel</p>
      </div>
      {navItems.map(({ to, label, icon: Icon }) => {
        const isActive =
          pathname === to || pathname.startsWith(to + "/");
        return (
          <Link
            key={to}
            to={to}
            className={cn(
              "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
            )}
          >
            <Icon className="h-4 w-4" />
            {label}
          </Link>
        );
      })}
    </nav>
  );
}
