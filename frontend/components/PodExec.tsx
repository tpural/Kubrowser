"use client";

import { useEffect, useRef, useState } from "react";
import { useTheme } from "next-themes";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

interface PodExecProps {
  podName: string;
  namespace: string;
  onClose: () => void;
}

export function PodExec({ podName, namespace, onClose }: PodExecProps) {
  const { resolvedTheme } = useTheme(); // Added useTheme hook
  console.log("[PodExec] Component rendered with podName:", podName, "namespace:", namespace);
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);

  // Update terminal theme when resolvedTheme changes
  useEffect(() => {
    if (!xtermRef.current) return;

    const isDark = resolvedTheme === "dark";
    xtermRef.current.options.theme = isDark
      ? {
        background: "#0a0a0a",
        foreground: "#e4e4e7",
        cursor: "#f4f4f5",
        black: "#18181b",
        red: "#ef4444",
        green: "#22c55e",
        yellow: "#eab308",
        blue: "#3b82f6",
        magenta: "#a855f7",
        cyan: "#06b6d4",
        white: "#f4f4f5",
        brightBlack: "#71717a",
        brightRed: "#f87171",
        brightGreen: "#4ade80",
        brightYellow: "#fbbf24",
        brightBlue: "#60a5fa",
        brightMagenta: "#c084fc",
        brightCyan: "#22d3ee",
        brightWhite: "#ffffff",
      }
      : {
        background: "#fafafa", // zinc-50
        foreground: "#18181b", // zinc-950
        cursor: "#18181b",
        black: "#18181b",
        red: "#ef4444",
        green: "#16a34a",
        yellow: "#ca8a04",
        blue: "#2563eb",
        magenta: "#9333ea",
        cyan: "#0891b2",
        white: "#f4f4f5",
        brightBlack: "#71717a",
        brightRed: "#ef4444",
        brightGreen: "#22c55e",
        brightYellow: "#eab308",
        brightBlue: "#3b82f6",
        brightMagenta: "#a855f7",
        brightCyan: "#06b6d4",
        brightWhite: "#ffffff",
      };
  }, [resolvedTheme]);

  const fitAddonRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const connectionTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    console.log("[PodExec] useEffect triggered for pod:", podName, "namespace:", namespace);

    // Handle resize - defined outside initTerminal so cleanup can access it
    const handleResize = () => {
      if (fitAddonRef.current && xtermRef.current) {
        fitAddonRef.current.fit();
        // Send resize to backend if connected
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
          const cols = xtermRef.current.cols;
          const rows = xtermRef.current.rows;
          // Note: The terminal executor handles resize internally via TTY
          // We don't need to send resize messages for exec
        }
      }
    };

    // Wait for the ref to be available - use a small timeout to ensure DOM is ready
    const initTerminal = () => {
      if (!terminalRef.current) {
        console.log("[PodExec] terminalRef.current is null, retrying in 50ms...");
        setTimeout(initTerminal, 50);
        return;
      }

      console.log("[PodExec] Initializing terminal...");
      // Initial theme setup (will be updated by the other useEffect)
      const isDark = resolvedTheme === "dark"; // Changed to use resolvedTheme
      const xterm = new XTerm({
        cursorBlink: true,
        fontSize: 14,
        fontFamily: "Monaco, Menlo, 'Ubuntu Mono', monospace",
        theme: isDark
          ? {
            background: "#0a0a0a",
            foreground: "#e4e4e7",
            cursor: "#f4f4f5",
            black: "#18181b",
            red: "#ef4444",
            green: "#22c55e",
            yellow: "#eab308",
            blue: "#3b82f6",
            magenta: "#a855f7",
            cyan: "#06b6d4",
            white: "#f4f4f5",
            brightBlack: "#71717a",
            brightRed: "#f87171",
            brightGreen: "#4ade80",
            brightYellow: "#fbbf24",
            brightBlue: "#60a5fa",
            brightMagenta: "#c084fc",
            brightCyan: "#22d3ee",
            brightWhite: "#ffffff",
          }
          : {
            background: "#fafafa", // zinc-50
            foreground: "#18181b", // zinc-950
            cursor: "#18181b",
            black: "#18181b",
            red: "#ef4444",
            green: "#16a34a",
            yellow: "#ca8a04",
            blue: "#2563eb",
            magenta: "#9333ea",
            cyan: "#0891b2",
            white: "#f4f4f5",
            brightBlack: "#71717a",
            brightRed: "#ef4444",
            brightGreen: "#22c55e",
            brightYellow: "#eab308",
            brightBlue: "#3b82f6",
            brightMagenta: "#a855f7",
            brightCyan: "#06b6d4",
            brightWhite: "#ffffff",
          },
      });

      const fitAddon = new FitAddon();
      xterm.loadAddon(fitAddon);
      xterm.open(terminalRef.current);
      fitAddon.fit();

      xtermRef.current = xterm;
      fitAddonRef.current = fitAddon;

      // Connect WebSocket
      console.log("[PodExec] Setting up WebSocket connection...");
      const getApiUrl = () => {
        if (process.env.NEXT_PUBLIC_API_URL) {
          const url = process.env.NEXT_PUBLIC_API_URL;
          // If it already has a protocol, extract host
          if (url.startsWith("http://") || url.startsWith("https://")) {
            return url.replace(/^https?:\/\//, "").replace(/\/$/, "");
          }
          return url;
        }
        return window.location.hostname + ":8080";
      };

      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const host = getApiUrl();
      const wsUrl = `${protocol}//${host}/api/v1/pods/${podName}/exec?namespace=${encodeURIComponent(namespace)}`;

      console.log("[PodExec] ===== WebSocket Connection Details =====");
      console.log("[PodExec] URL:", wsUrl);
      console.log("[PodExec] Protocol:", protocol);
      console.log("[PodExec] Host:", host);
      console.log("[PodExec] Pod Name:", podName);
      console.log("[PodExec] Namespace:", namespace);
      console.log("[PodExec] =========================================");

      // Show initial message in terminal
      if (xtermRef.current) {
        xtermRef.current.writeln(`\r\n\x1b[36mConnecting to pod ${podName} in namespace ${namespace}...\x1b[0m\r\n`);
      }

      // Set a connection timeout (shorter for faster feedback)
      connectionTimeoutRef.current = setTimeout(() => {
        if (wsRef.current && wsRef.current.readyState === WebSocket.CONNECTING) {
          console.error("[PodExec] Connection timeout after 5 seconds");
          wsRef.current.close();
          setConnected(false);
          if (xtermRef.current) {
            xtermRef.current.writeln("\r\n\x1b[31m✗ Connection timeout - unable to connect to backend\x1b[0m");
            xtermRef.current.writeln("\x1b[33mPlease check:\x1b[0m");
            xtermRef.current.writeln("  1. Backend server is running on " + host);
            xtermRef.current.writeln("  2. Pod exists and is in Running state");
            xtermRef.current.writeln("  3. Check browser console for WebSocket errors\r\n");
          }
        }
      }, 5000); // 5 second timeout for faster feedback

      console.log("[PodExec] Creating WebSocket instance...");
      let ws: WebSocket;
      try {
        ws = new WebSocket(wsUrl);
        console.log("[PodExec] WebSocket instance created, readyState:", ws.readyState);
        wsRef.current = ws; // Store immediately so timeout can check it
      } catch (err) {
        console.error("[PodExec] Failed to create WebSocket:", err);
        setConnected(false);
        if (xtermRef.current) {
          xtermRef.current.writeln(`\r\n\x1b[31m✗ Failed to create WebSocket connection: ${err}\x1b[0m\r\n`);
        }
        return;
      }

      console.log("[PodExec] Setting up WebSocket event handlers...");

      ws.onopen = () => {
        console.log("[PodExec] WebSocket connected");
        if (connectionTimeoutRef.current) {
          clearTimeout(connectionTimeoutRef.current);
          connectionTimeoutRef.current = null;
        }
        setConnected(true);
        if (xtermRef.current) {
          xtermRef.current.clear();
          xtermRef.current.writeln("\r\n\x1b[32mConnected to pod. Starting shell...\x1b[0m\r\n");
        }
      };

      ws.onmessage = (event) => {
        if (xtermRef.current) {
          if (typeof event.data === "string") {
            xtermRef.current.write(event.data);
          } else if (event.data instanceof ArrayBuffer) {
            xtermRef.current.write(new Uint8Array(event.data));
          } else if (event.data instanceof Blob) {
            const reader = new FileReader();
            reader.onload = () => {
              if (xtermRef.current && reader.result instanceof ArrayBuffer) {
                xtermRef.current.write(new Uint8Array(reader.result));
              }
            };
            reader.readAsArrayBuffer(event.data);
          }
        }
      };

      ws.onerror = (error) => {
        console.error("[PodExec] WebSocket error:", error);
        console.error("[PodExec] WebSocket state:", ws.readyState);
        console.error("[PodExec] WebSocket URL:", wsUrl);
        if (connectionTimeoutRef.current) {
          clearTimeout(connectionTimeoutRef.current);
          connectionTimeoutRef.current = null;
        }
        setConnected(false);
        if (xtermRef.current) {
          xtermRef.current.writeln("\r\n\x1b[31m✗ WebSocket connection error\x1b[0m");
          xtermRef.current.writeln("\x1b[33mCheck browser console (F12) for details\x1b[0m\r\n");
        }
      };

      ws.onclose = (event) => {
        console.log("[PodExec] WebSocket closed:", event.code, event.reason);
        console.log("[PodExec] Was clean:", event.wasClean);
        if (connectionTimeoutRef.current) {
          clearTimeout(connectionTimeoutRef.current);
          connectionTimeoutRef.current = null;
        }
        setConnected(false);
        if (xtermRef.current) {
          if (event.code !== 1000 && event.code !== 1001) {
            let errorMsg = `\r\n\x1b[31m✗ Connection closed`;
            if (event.code === 1006) {
              // Check for authentication failure
              const protocol = window.location.protocol === "https:" ? "https:" : "http:";
              const apiUrl = getApiUrl();
              fetch(`${protocol}//${apiUrl}/api/v1/namespaces`)
                .then((res) => {
                  if (res.status === 401) {
                    window.location.href = "/login";
                  }
                })
                .catch(() => { });

              errorMsg += " - Unable to connect to backend server";
              errorMsg += "\r\n\x1b[33mPossible causes:\x1b[0m";
              errorMsg += "\r\n  • Backend server not running";
              errorMsg += "\r\n  • Incorrect WebSocket URL";
              errorMsg += "\r\n  • Network/firewall blocking connection";
            } else if (event.code === 1002) {
              errorMsg += " - Protocol error";
            } else if (event.code === 1003) {
              errorMsg += " - Unsupported data";
            } else {
              errorMsg += ` (code: ${event.code})`;
            }
            if (event.reason) {
              errorMsg += `\r\n  Reason: ${event.reason}`;
            }
            errorMsg += "\x1b[0m\r\n";
            xtermRef.current.writeln(errorMsg);
          }
        }
      };

      // Send input to WebSocket
      xterm.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(data);
        }
      });

      window.addEventListener("resize", handleResize);
      // wsRef.current is already set above

      // Initial fit after a short delay to ensure DOM is ready
      setTimeout(() => {
        if (fitAddonRef.current) {
          fitAddonRef.current.fit();
        }
      }, 100);
    };

    // Start initialization
    initTerminal();

    // Return cleanup function
    return () => {
      window.removeEventListener("resize", handleResize);
      if (connectionTimeoutRef.current) {
        clearTimeout(connectionTimeoutRef.current);
        connectionTimeoutRef.current = null;
      }
      if (wsRef.current) {
        wsRef.current.close(1000, "Component unmounting");
        wsRef.current = null;
      }
      if (xtermRef.current) {
        xtermRef.current.dispose();
        xtermRef.current = null;
      }
    };
  }, [podName, namespace]);

  return (
    <Dialog open={true} onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        className="max-h-[90vh] flex flex-col"
        style={{ maxWidth: '65vw', width: '65vw' }}
      >
        <DialogHeader>
          <DialogTitle>
            Exec: {podName} ({namespace})
          </DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-hidden flex flex-col">
          <div className="flex-1 overflow-hidden bg-background rounded-md border p-2">
            <div ref={terminalRef} className="h-full w-full" />
          </div>
          {!connected && (
            <div className="text-sm text-muted-foreground mt-2">
              Connecting...
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
