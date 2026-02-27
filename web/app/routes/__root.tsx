import { Outlet, createRootRoute } from "@tanstack/react-router";
import { ErrorBoundary } from "~/components/shared/error-boundary";

export const Route = createRootRoute({
  component: () => (
    <ErrorBoundary>
      <Outlet />
    </ErrorBoundary>
  ),
});
