"use client";

import Link from "next/link";
import { Activity, Server, Clock, AlertCircle, ArrowRight } from "lucide-react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useEffect } from "react";
import { useAuthStore } from "@/lib/store";

export default function Home() {
  const queryClient = useQueryClient();
  const token = useAuthStore((state) => state.token);

  useEffect(() => {
    if (!token) return;

    let ws: WebSocket;
    
    const connect = () => {
      // Connect to WebSocket using the token as a query param or protocol. 
      // Assuming protocol for now as it is common for auth in WS.
      const wsUrl = process.env.NEXT_PUBLIC_API_URL?.replace("http", "ws") || "ws://localhost:8080";
      ws = new WebSocket(`${wsUrl}/api/v1/ws`, ["access_token", token]);

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          // If the event indicates a job or queue update, invalidate metrics
          if (data.type === "job_started" || data.type === "job_completed" || data.type === "job_failed" || data.type === "worker_status") {
             queryClient.invalidateQueries({ queryKey: ["system_metrics"] });
          }
        } catch (e) {
          console.error("WS parsing error", e);
        }
      };

      ws.onclose = () => {
        setTimeout(connect, 3000); // Reconnect
      };
    };

    connect();

    return () => {
      if (ws) ws.close();
    };
  }, [token, queryClient]);

  const { data: metricsData } = useQuery({
    queryKey: ["system_metrics"],
    queryFn: async () => {
      const res = await api.get("/metrics/system");
      return res.data.data;
    },
    // Still keep polling as fallback in case WS fails
    refetchInterval: 10000
  });

  const { data: orgsData } = useQuery({
    queryKey: ["orgs"],
    queryFn: async () => {
      const res = await api.get("/orgs");
      return res.data.data;
    }
  });

  const systemMetrics = metricsData || {
    ActiveWorkers: 0,
    DrainingWorkers: 0,
    TotalJobsRunning: 0,
    TotalJobsPending: 0,
    TotalQueues: 0
  };

  return (
    <div className="flex-col gap-6">
      <div className="flex justify-between items-center" style={{ marginBottom: "1.5rem" }}>
        <div>
          <h2 className="text-2xl">Dashboard Overview</h2>
          <p className="text-secondary">System status and global metrics</p>
        </div>
        <Link href="/orgs" className="btn btn-primary">
          Manage Organizations <ArrowRight size={16} />
        </Link>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))", gap: "1.5rem", marginBottom: "2rem" }}>
        <div className="glass-card">
          <div className="flex items-center gap-4" style={{ marginBottom: "1rem" }}>
            <div style={{ padding: "0.75rem", background: "rgba(59, 130, 246, 0.1)", borderRadius: "var(--radius-md)", color: "var(--primary)" }}>
              <Server size={24} />
            </div>
            <div>
              <p className="text-sm text-secondary">Active Workers</p>
              <h3 className="text-2xl">{systemMetrics.ActiveWorkers}</h3>
            </div>
          </div>
          <div className="flex justify-between items-center text-xs">
            <span className="badge badge-success">Online</span>
          </div>
        </div>

        <div className="glass-card">
          <div className="flex items-center gap-4" style={{ marginBottom: "1rem" }}>
            <div style={{ padding: "0.75rem", background: "rgba(245, 158, 11, 0.1)", borderRadius: "var(--radius-md)", color: "var(--warning)" }}>
              <Activity size={24} />
            </div>
            <div>
              <p className="text-sm text-secondary">Jobs Running</p>
              <h3 className="text-2xl">{systemMetrics.TotalJobsRunning}</h3>
            </div>
          </div>
          <div className="flex justify-between items-center text-xs">
            <span className="text-secondary">Total Queues: {systemMetrics.TotalQueues}</span>
          </div>
        </div>

        <div className="glass-card">
          <div className="flex items-center gap-4" style={{ marginBottom: "1rem" }}>
            <div style={{ padding: "0.75rem", background: "rgba(16, 185, 129, 0.1)", borderRadius: "var(--radius-md)", color: "var(--success)" }}>
              <Clock size={24} />
            </div>
            <div>
              <p className="text-sm text-secondary">Jobs Pending</p>
              <h3 className="text-2xl">{systemMetrics.TotalJobsPending}</h3>
            </div>
          </div>
        </div>

        <div className="glass-card">
          <div className="flex items-center gap-4" style={{ marginBottom: "1rem" }}>
            <div style={{ padding: "0.75rem", background: "rgba(239, 68, 68, 0.1)", borderRadius: "var(--radius-md)", color: "var(--error)" }}>
              <AlertCircle size={24} />
            </div>
            <div>
              <p className="text-sm text-secondary">Total Organizations</p>
              <h3 className="text-2xl">{orgsData?.length || 0}</h3>
            </div>
          </div>
          <div className="flex justify-between items-center text-xs">
            <Link href="/orgs" className="text-primary hover:underline">View details</Link>
          </div>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr", gap: "1.5rem" }}>
        <div className="glass-panel" style={{ padding: "1.5rem" }}>
          <h3 className="text-xl" style={{ marginBottom: "1.5rem" }}>System Overview</h3>
          
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: "1.5rem" }}>
            <div style={{ padding: "1rem", background: "rgba(255, 255, 255, 0.02)", borderRadius: "var(--radius-md)", border: "1px solid var(--border-color)" }}>
              <p className="text-sm text-secondary" style={{ marginBottom: "0.5rem" }}>Worker Health</p>
              <div className="flex justify-between items-end">
                <h4 className="text-xl">{systemMetrics.ActiveWorkers} <span className="text-sm text-secondary font-normal">Active</span></h4>
                <h4 className="text-xl">{systemMetrics.DrainingWorkers || 0} <span className="text-sm text-secondary font-normal">Draining</span></h4>
              </div>
              <div style={{ marginTop: "1rem", height: "4px", background: "var(--bg-lighter)", borderRadius: "2px", overflow: "hidden" }}>
                 <div style={{ width: "100%", height: "100%", background: "var(--primary)" }}></div>
              </div>
            </div>

            <div style={{ padding: "1rem", background: "rgba(255, 255, 255, 0.02)", borderRadius: "var(--radius-md)", border: "1px solid var(--border-color)" }}>
              <p className="text-sm text-secondary" style={{ marginBottom: "0.5rem" }}>Job Distribution</p>
              <div className="flex justify-between items-end">
                <h4 className="text-xl">{systemMetrics.TotalJobsRunning} <span className="text-sm text-secondary font-normal">Running</span></h4>
                <h4 className="text-xl">{systemMetrics.TotalJobsPending} <span className="text-sm text-secondary font-normal">Pending</span></h4>
              </div>
              <div style={{ marginTop: "1rem", height: "4px", background: "var(--bg-lighter)", borderRadius: "2px", overflow: "hidden", display: "flex" }}>
                 <div style={{ width: systemMetrics.TotalJobsRunning > 0 ? "50%" : "0%", height: "100%", background: "var(--warning)" }}></div>
                 <div style={{ width: systemMetrics.TotalJobsPending > 0 ? "50%" : "100%", height: "100%", background: "var(--success)" }}></div>
              </div>
            </div>
            
            <div style={{ padding: "1rem", background: "rgba(255, 255, 255, 0.02)", borderRadius: "var(--radius-md)", border: "1px solid var(--border-color)" }}>
              <p className="text-sm text-secondary" style={{ marginBottom: "0.5rem" }}>Queue Distribution</p>
              <div className="flex justify-between items-end">
                <h4 className="text-xl">{systemMetrics.TotalQueues} <span className="text-sm text-secondary font-normal">Active Queues</span></h4>
              </div>
              <div style={{ marginTop: "1rem", height: "4px", background: "var(--bg-lighter)", borderRadius: "2px", overflow: "hidden", display: "flex" }}>
                 <div style={{ width: "100%", height: "100%", background: "var(--secondary)" }}></div>
              </div>
            </div>
          </div>
          <p className="text-sm text-secondary" style={{ marginTop: "1.5rem" }}>Real-time metrics are updating automatically via WebSocket connection to the Job Scheduler backend.</p>
        </div>
      </div>
    </div>
  );
}
