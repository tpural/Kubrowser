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
        <div className="container mx-auto px-4 py-3">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold tracking-tight">Kubrowser</h1>
              <p className="text-sm text-muted-foreground">
                Browser-based kubectl terminal
              </p>
            </div>
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
          <div className="flex-1 overflow-hidden p-4 bg-background">
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
