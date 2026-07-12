"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState, useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useAuthStore } from "./store";

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => new QueryClient());
  const token = useAuthStore((state) => state.token);
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!token && pathname !== "/login" && pathname !== "/register") {
      router.push("/login");
    }
  }, [token, pathname, router]);

  return (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
}
