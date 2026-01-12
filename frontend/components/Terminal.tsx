"use client";

import { useEffect, useRef, useState } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

interface TerminalProps {
  sessionId?: string;
  onConnect?: (sessionId: string) => void;
  onDisconnect?: () => void;
  onError?: (error: string) => void;
  shouldDisconnect?: boolean;
}

export function Terminal({
  sessionId,
  onConnect,
  onDisconnect,
  onError,
  shouldDisconnect = false,
}: TerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const connectingRef = useRef(false);
  const initializedRef = useRef(false);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(
    sessionId || null
  );
  const [isLoading, setIsLoading] = useState(true);
  const [isConnecting, setIsConnecting] = useState(true);

  useEffect(() => {
    if (!terminalRef.current) return;

    // Prevent multiple initializations (handles React Strict Mode double-invoke)
    if (initializedRef.current) {
      return;
    }
    initializedRef.current = true;

    // Prevent multiple connections
    if (
      connectingRef.current ||
      wsRef.current?.readyState === WebSocket.OPEN ||
      wsRef.current?.readyState === WebSocket.CONNECTING
    ) {
      return;
    }

    connectingRef.current = true;

    // Initialize xterm with theme-aware colors
    const isDark = document.documentElement.classList.contains("dark");
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
            background: "#ffffff",
            foreground: "#18181b",
            cursor: "#09090b",
            black: "#09090b",
            red: "#dc2626",
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

    // Update terminal theme when dark mode changes
    const updateTerminalTheme = () => {
      if (!xtermRef.current) return;
      const isDark = document.documentElement.classList.contains("dark");
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
            background: "#ffffff",
            foreground: "#18181b",
            cursor: "#09090b",
            black: "#09090b",
            red: "#dc2626",
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
    };

    // Watch for theme changes
    const observer = new MutationObserver(updateTerminalTheme);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    // Handle window resize
    const handleResize = () => {
      if (fitAddonRef.current) {
        fitAddonRef.current.fit();
        // Send resize to backend
        if (
          wsRef.current &&
          currentSessionId &&
          wsRef.current.readyState === WebSocket.OPEN
        ) {
          const cols = xterm.cols;
          const rows = xterm.rows;
          fetch(`/api/v1/sessions/${currentSessionId}/resize`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ width: cols, height: rows }),
          }).catch((err) => console.error("Failed to resize terminal:", err));
        }
      }
    };

    window.addEventListener("resize", handleResize);

    // Connect WebSocket
    const connectWebSocket = () => {
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const host = process.env.NEXT_PUBLIC_API_URL
        ? process.env.NEXT_PUBLIC_API_URL.replace(/^https?:\/\//, "").replace(
            /\/$/,
            ""
          )
        : window.location.hostname + ":8080";
      const wsUrl = sessionId
        ? `${protocol}//${host}/api/v1/ws?session_id=${sessionId}&reconnect=true`
        : `${protocol}//${host}/api/v1/ws`;

      const ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        connectingRef.current = false;
        setIsConnecting(false);
        // Connection opened, wait for session message if new session
        if (!sessionId) {
          // Will receive session info in first message
        }
      };

      let sessionReceived = false;

      ws.onmessage = (event) => {
        // First message might be session info (text), rest are terminal data (binary)
        if (!sessionReceived && typeof event.data === "string") {
          try {
            const data = JSON.parse(event.data);
            if (data.type === "session" && data.session_id) {
              sessionReceived = true;
              setCurrentSessionId(data.session_id);
              setIsLoading(false);
              if (onConnect) {
                onConnect(data.session_id);
              }
              return;
            }
          } catch {
            // Not JSON, treat as text terminal data
          }
        }

        // Terminal data
        if (xtermRef.current) {
          setIsLoading(false);
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

      ws.onerror = () => {
        // WebSocket error event doesn't provide detailed error info
        // Log for debugging only (less noisy than console.error)
        // Actual error details will be in onclose handler
        if (process.env.NODE_ENV === "development") {
          const readyStateText = ws.readyState === WebSocket.CONNECTING ? "CONNECTING" : 
                                ws.readyState === WebSocket.OPEN ? "OPEN" :
                                ws.readyState === WebSocket.CLOSING ? "CLOSING" :
                                ws.readyState === WebSocket.CLOSED ? "CLOSED" : "UNKNOWN";
          console.warn(
            `[WebSocket] Connection error. URL: ${wsUrl}, State: ${readyStateText} (${ws.readyState})`
          );
        }
        // Don't call onError here - wait for onclose which has actual error details
      };

      ws.onclose = (event) => {
        connectingRef.current = false;
        setIsConnecting(false);
        setIsLoading(false);
        
        // Log connection closure details (only in development)
        if (process.env.NODE_ENV === "development") {
          console.log("[WebSocket] Connection closed:", {
            code: event.code,
            reason: event.reason || "(no reason provided)",
            wasClean: event.wasClean,
            url: wsUrl,
          });
        }
        
        // Provide detailed error information based on close code
        // Only report errors for abnormal closures (not normal shutdowns)
        if (event.code !== 1000 && event.code !== 1001) {
          let errorMessage = "";
          
          switch (event.code) {
            case 1006:
              errorMessage = `Unable to connect to backend server at ${host}. Please ensure the backend is running (try: make dev or start the backend server on port 8080).`;
              break;
            case 1002:
              errorMessage = "WebSocket protocol error occurred.";
              break;
            case 1003:
              errorMessage = "Invalid data received from server.";
              break;
            case 1008:
              errorMessage = `Connection rejected: ${event.reason || "Policy violation"}`;
              break;
            case 1011:
              errorMessage = `Server error: ${event.reason || "Internal server error"}`;
              break;
            default:
              errorMessage = `Connection closed unexpectedly (code: ${event.code})${event.reason ? `: ${event.reason}` : ""}`;
          }
          
          if (errorMessage && onError) {
            onError(errorMessage);
          }
          
          if (onDisconnect) {
            onDisconnect();
          }
        }
      };

      // Send input to WebSocket
      xterm.onData((data) => {
        // Don't send data if disconnected
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
          ws.send(data);
        }
      });

      wsRef.current = ws;
    };

    connectWebSocket();

    return () => {
      initializedRef.current = false;
      connectingRef.current = false;
      window.removeEventListener("resize", handleResize);
      observer.disconnect();
      if (wsRef.current) {
        wsRef.current.close(1000, "Component unmounting");
        wsRef.current = null;
      }
      if (xtermRef.current) {
        xtermRef.current.dispose();
        xtermRef.current = null;
      }
    };
    // Only run once on mount - don't depend on sessionId to prevent reconnections
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Handle disconnect request
  useEffect(() => {
    if (shouldDisconnect && wsRef.current) {
      const ws = wsRef.current;
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        // Close the WebSocket connection
        ws.close(1000, "User requested disconnect");
        wsRef.current = null;
        setIsConnecting(false);
        setIsLoading(false);
        
        // Clear terminal and show disconnected message
        if (xtermRef.current) {
          xtermRef.current.clear();
          xtermRef.current.writeln("\r\n\x1b[31mDisconnected\x1b[0m - Connection closed by user.");
          xtermRef.current.writeln("Press Reconnect to establish a new connection.");
        }
        
        if (onDisconnect) {
          onDisconnect();
        }
      }
    }
  }, [shouldDisconnect, onDisconnect]);

  return (
    <div className="h-full w-full relative">
      {(isLoading || isConnecting) && (
        <div className="absolute inset-0 z-10 flex items-center justify-center bg-background/80 backdrop-blur-sm">
          <div className="flex flex-col items-center gap-4">
            <div className="relative">
              <div className="h-12 w-12 rounded-full border-4 border-muted"></div>
              <div className="absolute top-0 left-0 h-12 w-12 rounded-full border-4 border-primary border-t-transparent animate-spin"></div>
            </div>
            <p className="text-sm text-muted-foreground">
              {isConnecting ? "Connecting..." : "Loading terminal..."}
            </p>
          </div>
        </div>
      )}
      <div ref={terminalRef} className="h-full w-full" />
    </div>
  );
}
