"use client";

import { usePathname, useRouter } from "next/navigation";
import Link from "next/link";
import { LayoutDashboard, List, Activity, Settings, LogOut } from "lucide-react";
import { useAuthStore } from "./store";
import { useEffect } from "react";

export function ClientLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const token = useAuthStore((state) => state.token);
  const logout = useAuthStore((state) => state.logout);

  const isAuthPage = pathname === "/login" || pathname === "/register";

  useEffect(() => {
    if (!isAuthPage && !token) {
      router.replace("/login");
    }
  }, [token, isAuthPage, router]);

  if (isAuthPage) {
    return <main style={{ flex: 1 }}>{children}</main>;
  }

  // While redirecting, show nothing to avoid flash
  if (!token) {
    return null;
  }

  const handleLogout = () => {
    logout();
    router.replace("/login");
  };

  return (
    <>
      {/* Sidebar */}
      <aside className="glass-panel" style={{ width: "260px", margin: "1rem", display: "flex", flexDirection: "column" }}>
        <div style={{ padding: "1.5rem", borderBottom: "1px solid var(--border)" }}>
          <div className="flex items-center gap-2">
            <div style={{ width: "32px", height: "32px", background: "var(--primary)", borderRadius: "var(--radius-md)", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "var(--shadow-glow)" }}>
              <Activity size={18} color="white" />
            </div>
            <h1 className="text-lg">Job Scheduler</h1>
          </div>
        </div>
        
        <nav style={{ padding: "1rem", flex: 1, display: "flex", flexDirection: "column", gap: "0.5rem" }}>
          <Link href="/" className="btn btn-outline" style={{ justifyContent: "flex-start", border: "none", background: pathname === "/" ? "var(--surface-hover)" : "transparent" }}>
            <LayoutDashboard size={18} /> Dashboard
          </Link>
          <Link href="/orgs" className="btn btn-outline" style={{ justifyContent: "flex-start", border: "none", background: pathname === "/orgs" ? "var(--surface-hover)" : "transparent" }}>
            <List size={18} /> Organizations
          </Link>
        </nav>
        
        <div style={{ padding: "1rem", borderTop: "1px solid var(--border)" }}>
          <button onClick={handleLogout} className="btn btn-outline" style={{ width: "100%", justifyContent: "flex-start", border: "none" }}>
            <LogOut size={18} /> Logout
          </button>
        </div>
      </aside>
      
      {/* Main Content */}
      <main style={{ flex: 1, padding: "1rem 2rem 1rem 0", display: "flex", flexDirection: "column" }}>
        <header className="glass-panel" style={{ padding: "1rem 2rem", marginBottom: "1.5rem", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <div className="text-sm text-secondary">
            Project: <span className="text-primary font-medium">Default</span>
          </div>
          <div className="flex items-center gap-4">
            <div className="badge badge-success">System Healthy</div>
            <div style={{ width: "32px", height: "32px", borderRadius: "50%", background: "var(--surface-hover)", border: "1px solid var(--border)", display: "flex", alignItems: "center", justifyContent: "center" }}>
              <span className="text-xs">U</span>
            </div>
          </div>
        </header>
        
        <div style={{ flex: 1, overflowY: "auto" }}>
          {children}
        </div>
      </main>
    </>
  );
}
