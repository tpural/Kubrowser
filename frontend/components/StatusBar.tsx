"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

interface StatusBarProps {
  connected: boolean;
  sessionId?: string;
  podName?: string;
  onReconnect?: () => void;
  onDisconnect?: () => void;
}

export function StatusBar({
  connected,
  sessionId,
  podName,
  onReconnect,
  onDisconnect,
}: StatusBarProps) {
  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-3">
        <Badge 
          variant={connected ? "default" : "destructive"}
          className="font-medium"
        >
          <span className={`inline-block w-2 h-2 rounded-full mr-1.5 ${
            connected ? "bg-green-500 animate-pulse" : "bg-red-500"
          }`}></span>
          {connected ? "Connected" : "Disconnected"}
        </Badge>
        {sessionId && (
          <div className="flex items-center gap-2 text-sm">
            <span className="text-muted-foreground">Session:</span>
            <code className="px-2 py-0.5 bg-muted rounded text-xs font-mono">
              {sessionId.substring(0, 8)}...
            </code>
          </div>
        )}
        {podName && (
          <div className="flex items-center gap-2 text-sm">
            <span className="text-muted-foreground">Pod:</span>
            <code className="px-2 py-0.5 bg-muted rounded text-xs font-mono">
              {podName}
            </code>
          </div>
        )}
      </div>
      <div className="flex gap-2">
        {!connected && onReconnect && (
          <Button size="sm" onClick={onReconnect}>
            Reconnect
          </Button>
        )}
        {connected && onDisconnect && (
          <Button size="sm" variant="outline" onClick={onDisconnect}>
            Disconnect
          </Button>
        )}
      </div>
    </div>
  );
}
