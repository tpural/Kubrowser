"use client";

import { useState } from "react";
import { Terminal } from "@/components/Terminal";
import { StatusBar } from "@/components/StatusBar";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Card } from "@/components/ui/card";

export default function Home() {
  const [connected, setConnected] = useState(false);
  const [sessionId, setSessionId] = useState<string | undefined>();
  const [podName] = useState<string | undefined>();
  const [shouldDisconnect, setShouldDisconnect] = useState(false);

  const handleConnect = (newSessionId: string) => {
    setSessionId(newSessionId);
    setConnected(true);
    setShouldDisconnect(false); // Reset disconnect flag when reconnecting
  };

  const handleDisconnect = () => {
    setShouldDisconnect(true);
    setConnected(false);
  };

  const handleReconnect = () => {
    // Terminal component will handle reconnection
    window.location.reload();
  };

  return (
    <div className="flex h-screen flex-col bg-background">
      <header className="border-b bg-card/50 backdrop-blur supports-[backdrop-filter]:bg-card/50">
        <div className="flex items-center justify-between py-3">
          <div className="flex items-center gap-4 pl-4">
            <div className="relative h-16 w-16 flex-shrink-0">
              <img
                src="/logo.svg"
                alt="Kubrowser Logo"
                className="h-full w-full object-contain"
              />
            </div>
            <div>
              <h1 className="text-2xl font-bold tracking-tight text-[#326CE5]" style={{ fontFamily: 'var(--font-jetbrains-mono)' }}>
                Kubrowser
              </h1>
              <p className="text-sm text-muted-foreground">
                Browser-based kubectl terminal
              </p>
            </div>
          </div>
          <div className="pr-4">
            <ThemeToggle />
          </div>
        </div>
      </header>
      <div className="flex-1 overflow-hidden p-4">
        <Card className="h-full flex flex-col shadow-lg border-2">
          <div className="p-3 border-b bg-muted/30">
            <StatusBar
              connected={connected}
              sessionId={sessionId}
              podName={podName}
              onReconnect={handleReconnect}
              onDisconnect={handleDisconnect}
            />
          </div>
          <div className="flex-1 overflow-hidden p-4 bg-background flex flex-col">
            <Terminal
              sessionId={sessionId}
              onConnect={handleConnect}
              onDisconnect={handleDisconnect}
              onError={(error) => console.error("Terminal error:", error)}
              shouldDisconnect={shouldDisconnect}
            />
          </div>
        </Card>
      </div>
    </div>
  );
}
