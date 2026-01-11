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
}

export function Terminal({
  sessionId,
  onConnect,
  onDisconnect,
  onError,
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

    // Initialize xterm
    const xterm = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: "Monaco, Menlo, 'Ubuntu Mono', monospace",
      theme: {
        background: "#1e1e1e",
        foreground: "#d4d4d4",
        cursor: "#aeafad",
      },
    });

    const fitAddon = new FitAddon();
    xterm.loadAddon(fitAddon);
    xterm.open(terminalRef.current);
    fitAddon.fit();

    xtermRef.current = xterm;
    fitAddonRef.current = fitAddon;

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
        connectingRef.current = false;
        console.error("WebSocket error:", error);
        if (onError) {
          onError("Connection error occurred");
        }
      };

      ws.onclose = (event) => {
        connectingRef.current = false;
        // Don't trigger disconnect on normal closure or if we're reconnecting
        if (event.code !== 1000 && event.code !== 1001) {
          if (onDisconnect) {
            onDisconnect();
          }
        }
      };

      // Send input to WebSocket
      xterm.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
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

  return (
    <div className="h-full w-full">
      <div ref={terminalRef} className="h-full w-full" />
    </div>
  );
}
