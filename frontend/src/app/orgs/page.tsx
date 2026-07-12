"use client";

import { useState } from "react";
import { Users, Building, Settings, Plus, Search, Trash2, Edit } from "lucide-react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

export default function Organizations() {
  const queryClient = useQueryClient();
  const [searchTerm, setSearchTerm] = useState("");
  
  // Modals state
  const [showModal, setShowModal] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [currentOrgId, setCurrentOrgId] = useState<string | null>(null);
  const [formData, setFormData] = useState({ name: "", slug: "", description: "" });
  
  // Fetch orgs
  const { data: orgsData, isLoading } = useQuery({
    queryKey: ["orgs"],
    queryFn: async () => {
      const res = await api.get("/orgs");
      return res.data.data;
    }
  });

  const createMutation = useMutation({
    mutationFn: (newOrg: any) => api.post("/orgs", newOrg),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["orgs"] });
      closeModal();
    }
  });

  const updateMutation = useMutation({
    mutationFn: (org: any) => api.put(`/orgs/${org.id}`, { name: org.name, description: org.description }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["orgs"] });
      closeModal();
    }
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/orgs/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["orgs"] });
    }
  });

  const openCreateModal = () => {
    setModalMode("create");
    setFormData({ name: "", slug: "", description: "" });
    setShowModal(true);
  };

  const openEditModal = (org: any) => {
    setModalMode("edit");
    setCurrentOrgId(org.id);
    setFormData({ name: org.name, slug: org.slug, description: org.description });
    setShowModal(true);
  };

  const closeModal = () => {
    setShowModal(false);
    setCurrentOrgId(null);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (modalMode === "create") {
      createMutation.mutate(formData);
    } else {
      updateMutation.mutate({ id: currentOrgId, ...formData });
    }
  };

  const orgs = orgsData || [];
  const filteredOrgs = orgs.filter((org: any) => org.name.toLowerCase().includes(searchTerm.toLowerCase()));

  return (
    <div className="flex-col gap-6" style={{ position: "relative" }}>
      <div className="flex justify-between items-center" style={{ marginBottom: "1.5rem" }}>
        <div>
          <h2 className="text-2xl">Organizations</h2>
          <p className="text-secondary">Manage your workspaces and team members</p>
        </div>
        <button onClick={openCreateModal} className="btn btn-primary">
          <Plus size={16} /> New Organization
        </button>
      </div>

      <div className="glass-panel" style={{ padding: "1.5rem", marginBottom: "2rem" }}>
        <div className="flex justify-between items-center" style={{ marginBottom: "1.5rem" }}>
          <div className="flex items-center gap-2" style={{ background: "var(--surface)", padding: "0.5rem 1rem", borderRadius: "var(--radius-md)", border: "1px solid var(--border)", width: "300px" }}>
            <Search size={16} className="text-secondary" />
            <input 
              type="text" 
              placeholder="Search organizations..." 
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              style={{ background: "transparent", border: "none", color: "var(--text)", outline: "none", width: "100%", fontSize: "0.875rem" }}
            />
          </div>
          <div className="flex gap-2">
            <select className="btn btn-outline" style={{ background: "var(--surface)", border: "1px solid var(--border)" }}>
              <option>All Status</option>
              <option>Active</option>
              <option>Archived</option>
            </select>
          </div>
        </div>

        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ borderBottom: "1px solid var(--border)", textAlign: "left" }}>
              <th style={{ padding: "1rem 0", color: "var(--text-secondary)", fontWeight: 500 }}>Organization Name</th>
              <th style={{ padding: "1rem 0", color: "var(--text-secondary)", fontWeight: 500 }}>Created At</th>
              <th style={{ padding: "1rem 0", color: "var(--text-secondary)", fontWeight: 500 }}>Status</th>
              <th style={{ padding: "1rem 0", color: "var(--text-secondary)", fontWeight: 500, textAlign: "right" }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={4} style={{ padding: "1rem", textAlign: "center" }}>Loading...</td></tr>
            ) : filteredOrgs.length === 0 ? (
              <tr><td colSpan={4} style={{ padding: "1rem", textAlign: "center" }}>No organizations found.</td></tr>
            ) : (
              filteredOrgs.map((org: any) => (
                <tr key={org.id} style={{ borderBottom: "1px solid var(--border)", transition: "background 0.2s" }} className="hover:bg-surface">
                  <td style={{ padding: "1rem 0" }}>
                    <div className="flex items-center gap-3">
                      <div style={{ padding: "0.5rem", background: "var(--surface-hover)", borderRadius: "var(--radius-sm)" }}>
                        <Building size={16} className="text-primary" />
                      </div>
                      <div className="flex flex-col">
                        <span className="font-medium">{org.name}</span>
                        <span className="text-xs text-secondary">{org.description || org.slug}</span>
                      </div>
                    </div>
                  </td>
                  <td style={{ padding: "1rem 0", color: "var(--text-secondary)", fontSize: "0.875rem" }}>
                    {new Date(org.created_at).toLocaleDateString()}
                  </td>
                  <td style={{ padding: "1rem 0" }}>
                    <span className="badge badge-success">Active</span>
                  </td>
                  <td style={{ padding: "1rem 0", textAlign: "right" }}>
                    <div className="flex justify-end gap-2">
                      <button onClick={() => openEditModal(org)} className="btn btn-outline" style={{ padding: "0.25rem 0.5rem" }}>
                        <Edit size={14} />
                      </button>
                      <button onClick={() => { if(confirm("Are you sure?")) deleteMutation.mutate(org.id) }} className="btn btn-outline" style={{ padding: "0.25rem 0.5rem", borderColor: "var(--error)", color: "var(--error)" }}>
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {showModal && (
        <div style={{ position: "fixed", top: 0, left: 0, right: 0, bottom: 0, background: "rgba(0,0,0,0.5)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 100 }}>
          <div className="glass-panel" style={{ padding: "2rem", width: "100%", maxWidth: "400px", display: "flex", flexDirection: "column", gap: "1rem" }}>
            <h3 className="text-xl">{modalMode === "create" ? "New Organization" : "Edit Organization"}</h3>
            <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
              <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                <label className="text-sm font-medium">Name</label>
                <input 
                  type="text" required value={formData.name} onChange={(e) => setFormData({...formData, name: e.target.value})}
                  className="glass-panel" style={{ padding: "0.5rem", border: "1px solid var(--border)", background: "var(--surface)", color: "var(--text)" }}
                />
              </div>
              {modalMode === "create" && (
                <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                  <label className="text-sm font-medium">Slug</label>
                  <input 
                    type="text" required value={formData.slug} onChange={(e) => setFormData({...formData, slug: e.target.value})}
                    className="glass-panel" style={{ padding: "0.5rem", border: "1px solid var(--border)", background: "var(--surface)", color: "var(--text)" }}
                  />
                </div>
              )}
              <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                <label className="text-sm font-medium">Description</label>
                <textarea 
                  value={formData.description} onChange={(e) => setFormData({...formData, description: e.target.value})}
                  className="glass-panel" style={{ padding: "0.5rem", border: "1px solid var(--border)", background: "var(--surface)", color: "var(--text)", minHeight: "80px" }}
                />
              </div>
              <div className="flex justify-end gap-2" style={{ marginTop: "1rem" }}>
                <button type="button" onClick={closeModal} className="btn btn-outline">Cancel</button>
                <button type="submit" className="btn btn-primary" disabled={createMutation.isPending || updateMutation.isPending}>
                  {createMutation.isPending || updateMutation.isPending ? "Saving..." : "Save"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
