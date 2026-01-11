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
    <Card className="p-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Badge variant={connected ? "default" : "destructive"}>
            {connected ? "Connected" : "Disconnected"}
          </Badge>
          {sessionId && (
            <span className="text-sm text-muted-foreground">
              Session: {sessionId.substring(0, 8)}...
            </span>
          )}
          {podName && (
            <span className="text-sm text-muted-foreground">
              Pod: {podName}
            </span>
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
    </Card>
  );
}
