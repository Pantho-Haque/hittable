"use client";

import { Shield } from "lucide-react";
import { useState } from "react";
import { ModalShell } from "@/components";
import { useDataContext } from "@/context/dataContext";

type AuthPreset = "none" | "bearer" | "basic" | "apikey";

const AUTH_PRESETS: { value: AuthPreset; label: string; description: string }[] = [
  { value: "none", label: "No Auth", description: "No authentication" },
  { value: "bearer", label: "Bearer Token", description: "Adds Authorization: Bearer <token>" },
  { value: "basic", label: "Basic Auth", description: "Adds Authorization: Basic <base64(user:pass)>" },
  { value: "apikey", label: "API Key", description: "Adds a custom header with your API key" },
];

export default function AuthModal() {
  const { formInput, setFormInput } = useDataContext();
  const [open, setOpen] = useState(false);
  const [selectedPreset, setSelectedPreset] = useState<AuthPreset>("none");
  const [token, setToken] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [apiKeyHeader, setApiKeyHeader] = useState("X-API-Key");
  const [apiKeyValue, setApiKeyValue] = useState("");

  const getCurrentHeaders = (): Record<string, string> => {
    try {
      return JSON.parse(formInput.headers || "{}");
    } catch {
      return {};
    }
  };

  const applyAuth = () => {
    const headers = getCurrentHeaders();

    // Remove any existing auth headers
    delete headers["Authorization"];
    delete headers[apiKeyHeader];

    const newHeaders = { ...headers };

    switch (selectedPreset) {
      case "bearer":
        if (token) {
          newHeaders["Authorization"] = `Bearer ${token}`;
        }
        break;
      case "basic":
        if (username || password) {
          const encoded = btoa(`${username}:${password}`);
          newHeaders["Authorization"] = `Basic ${encoded}`;
        }
        break;
      case "apikey":
        if (apiKeyHeader && apiKeyValue) {
          newHeaders[apiKeyHeader] = apiKeyValue;
        }
        break;
    }

    setFormInput({
      ...formInput,
      headers: JSON.stringify(newHeaders, null, "\t"),
    });
    setOpen(false);
  };

  const clearAuth = () => {
    const headers = getCurrentHeaders();
    delete headers["Authorization"];
    delete headers[apiKeyHeader];
    setFormInput({
      ...formInput,
      headers: JSON.stringify(headers, null, "\t"),
    });
    setSelectedPreset("none");
    setOpen(false);
  };

  return (
    <>
      <button
        onClick={(e) => {
          e.stopPropagation();
          setOpen(true);
        }}
        title="Auth"
        className="modal-button-mini"
      >
        <Shield size={14} />
      </button>

      {open && (
        <ModalShell
          title="Authentication"
          subtitle="Add auth headers to your request"
          onClose={() => setOpen(false)}
          size="md"
        >
          <div className="flex flex-col gap-4">
            {/* Preset selector */}
            <div className="flex flex-col gap-2">
              <label className="text-[9px] tracking-[0.2em] uppercase text-white/30">
                Type
              </label>
              <div className="grid grid-cols-2 gap-2">
                {AUTH_PRESETS.map((preset) => (
                  <button
                    key={preset.value}
                    onClick={() => setSelectedPreset(preset.value)}
                    className={`flex flex-col items-start p-3 rounded-lg border transition-all cursor-pointer text-left ${
                      selectedPreset === preset.value
                        ? "border-cyan-500/40 bg-cyan-500/10"
                        : "border-white/10 bg-white/5 hover:border-white/20"
                    }`}
                  >
                    <span
                      className={`text-xs font-semibold ${
                        selectedPreset === preset.value ? "text-cyan-400" : "text-white/60"
                      }`}
                    >
                      {preset.label}
                    </span>
                    <span className="text-[10px] text-white/30 mt-0.5">
                      {preset.description}
                    </span>
                  </button>
                ))}
              </div>
            </div>

            {/* Preset-specific fields */}
            {selectedPreset === "bearer" && (
              <div className="flex flex-col gap-2">
                <label className="text-[9px] tracking-[0.2em] uppercase text-white/30">
                  Token
                </label>
                <input
                  type="text"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="eyJhbGciOiJIUzI1NiIs..."
                  className="w-full border-b border-cyan-500/30 bg-transparent outline-none py-2 text-xs text-white/80 placeholder-white/20 focus:border-cyan-400 transition-colors"
                />
              </div>
            )}

            {selectedPreset === "basic" && (
              <div className="flex flex-col gap-3">
                <div className="flex flex-col gap-2">
                  <label className="text-[9px] tracking-[0.2em] uppercase text-white/30">
                    Username
                  </label>
                  <input
                    type="text"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="admin"
                    className="w-full border-b border-cyan-500/30 bg-transparent outline-none py-2 text-xs text-white/80 placeholder-white/20 focus:border-cyan-400 transition-colors"
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <label className="text-[9px] tracking-[0.2em] uppercase text-white/30">
                    Password
                  </label>
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="••••••••"
                    className="w-full border-b border-cyan-500/30 bg-transparent outline-none py-2 text-xs text-white/80 placeholder-white/20 focus:border-cyan-400 transition-colors"
                  />
                </div>
              </div>
            )}

            {selectedPreset === "apikey" && (
              <div className="flex flex-col gap-3">
                <div className="flex flex-col gap-2">
                  <label className="text-[9px] tracking-[0.2em] uppercase text-white/30">
                    Header Name
                  </label>
                  <input
                    type="text"
                    value={apiKeyHeader}
                    onChange={(e) => setApiKeyHeader(e.target.value)}
                    placeholder="X-API-Key"
                    className="w-full border-b border-cyan-500/30 bg-transparent outline-none py-2 text-xs text-white/80 placeholder-white/20 focus:border-cyan-400 transition-colors"
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <label className="text-[9px] tracking-[0.2em] uppercase text-white/30">
                    Value
                  </label>
                  <input
                    type="text"
                    value={apiKeyValue}
                    onChange={(e) => setApiKeyValue(e.target.value)}
                    placeholder="your-api-key"
                    className="w-full border-b border-cyan-500/30 bg-transparent outline-none py-2 text-xs text-white/80 placeholder-white/20 focus:border-cyan-400 transition-colors"
                  />
                </div>
              </div>
            )}

            {selectedPreset === "none" && (
              <div className="flex items-center justify-center py-6 text-white/20 text-xs">
                No authentication will be added to requests
              </div>
            )}
          </div>

          <div className="flex justify-between gap-2 pt-2">
            <button
              onClick={clearAuth}
              className="px-4 py-1.5 text-xs rounded-md border border-red-500/20 text-red-400/60 hover:bg-red-500/10 hover:text-red-400 transition-colors cursor-pointer"
            >
              Clear Auth
            </button>
            <div className="flex gap-2">
              <button
                onClick={() => setOpen(false)}
                className="px-4 py-1.5 text-xs rounded-md border border-white/10 text-white/40 hover:bg-white/5 hover:text-white/70 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={applyAuth}
                className="px-4 py-1.5 text-xs rounded-md font-bold cursor-pointer active:scale-95 transition-all"
                style={{
                  background: "rgba(0,229,204,0.15)",
                  border: "1px solid rgba(0,229,204,0.3)",
                  color: "#00e5cc",
                }}
              >
                Apply
              </button>
            </div>
          </div>
        </ModalShell>
      )}
    </>
  );
}
