"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Activity } from "lucide-react";
import { api } from "@/lib/api";
import { useAuthStore } from "@/lib/store";

export default function Register() {
  const router = useRouter();
  const setToken = useAuthStore((state) => state.setToken);
  
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    
    try {
      await api.post("/auth/register", { first_name: firstName, last_name: lastName, email, password });
      const loginRes = await api.post("/auth/login", { email, password });
      setToken(loginRes.data.data.access_token);
      router.push("/");
    } catch (err: any) {
      setError(err.response?.data?.error?.message || "Failed to register");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex" style={{ minHeight: "100vh", alignItems: "center", justifyContent: "center" }}>
      <div className="glass-panel" style={{ padding: "2rem", width: "100%", maxWidth: "400px", display: "flex", flexDirection: "column", gap: "1.5rem" }}>
        <div className="flex flex-col items-center gap-2" style={{ marginBottom: "1rem" }}>
          <div style={{ width: "48px", height: "48px", background: "var(--primary)", borderRadius: "var(--radius-md)", display: "flex", alignItems: "center", justifyContent: "center", boxShadow: "var(--shadow-glow)" }}>
            <Activity size={24} color="white" />
          </div>
          <h2 className="text-2xl">Create Account</h2>
          <p className="text-secondary">Join Job Scheduler today</p>
        </div>
        
        {error && (
          <div style={{ padding: "0.75rem", background: "rgba(239, 68, 68, 0.1)", color: "var(--error)", borderRadius: "var(--radius-sm)", fontSize: "0.875rem" }}>
            {error}
          </div>
        )}
        
        <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
          <div className="flex gap-4">
            <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem", flex: 1 }}>
              <label className="text-sm font-medium">First Name</label>
              <input 
                type="text" 
                required
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                className="glass-panel"
                style={{ padding: "0.75rem", border: "1px solid var(--border)", background: "var(--surface)", color: "var(--text)", outline: "none", borderRadius: "var(--radius-sm)", width: "100%" }}
              />
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem", flex: 1 }}>
              <label className="text-sm font-medium">Last Name</label>
              <input 
                type="text" 
                required
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                className="glass-panel"
                style={{ padding: "0.75rem", border: "1px solid var(--border)", background: "var(--surface)", color: "var(--text)", outline: "none", borderRadius: "var(--radius-sm)", width: "100%" }}
              />
            </div>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
            <label className="text-sm font-medium">Email</label>
            <input 
              type="email" 
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="glass-panel"
              style={{ padding: "0.75rem", border: "1px solid var(--border)", background: "var(--surface)", color: "var(--text)", outline: "none", borderRadius: "var(--radius-sm)" }}
            />
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
            <label className="text-sm font-medium">Password</label>
            <input 
              type="password" 
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="glass-panel"
              style={{ padding: "0.75rem", border: "1px solid var(--border)", background: "var(--surface)", color: "var(--text)", outline: "none", borderRadius: "var(--radius-sm)" }}
            />
          </div>
          <button type="submit" className="btn btn-primary" disabled={loading} style={{ justifyContent: "center", marginTop: "1rem" }}>
            {loading ? "Creating account..." : "Sign Up"}
          </button>
        </form>
        
        <div style={{ textAlign: "center", fontSize: "0.875rem" }}>
          <span className="text-secondary">Already have an account? </span>
          <Link href="/login" className="text-primary hover:underline">Sign in</Link>
        </div>
      </div>
    </div>
  );
}
