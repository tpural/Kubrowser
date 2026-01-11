"use client";

import { useState } from "react";
import { Terminal } from "@/components/Terminal";
import { StatusBar } from "@/components/StatusBar";
import { Card } from "@/components/ui/card";

export default function Home() {
  const [connected, setConnected] = useState(false);
  const [sessionId, setSessionId] = useState<string | undefined>();
  const [podName] = useState<string | undefined>();

  const handleConnect = (newSessionId: string) => {
    setSessionId(newSessionId);
    setConnected(true);
  };

  const handleDisconnect = () => {
    setConnected(false);
  };

  const handleReconnect = () => {
    // Terminal component will handle reconnection
    window.location.reload();
  };

  return (
    <div className="flex h-screen flex-col bg-background">
      <header className="border-b p-4">
        <h1 className="text-2xl font-bold">Kubrowser</h1>
        <p className="text-sm text-muted-foreground">
          Browser-based kubectl terminal
        </p>
      </header>
      <div className="flex-1 overflow-hidden p-4">
        <Card className="h-full flex flex-col">
          <div className="p-2 border-b">
            <StatusBar
              connected={connected}
              sessionId={sessionId}
              podName={podName}
              onReconnect={handleReconnect}
              onDisconnect={handleDisconnect}
            />
          </div>
          <div className="flex-1 overflow-hidden p-4">
            <Terminal
              sessionId={sessionId}
              onConnect={handleConnect}
              onDisconnect={handleDisconnect}
              onError={(error) => console.error("Terminal error:", error)}
            />
          </div>
        </Card>
      </div>
    </div>
  );
}
