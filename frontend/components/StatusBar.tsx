"use client";

import { motion } from "framer-motion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Wifi, WifiOff, Terminal, Box, RefreshCw, Power } from "lucide-react";

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
      <div className="flex items-center gap-4">
        {/* Connection Status */}
        <motion.div
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          key={connected ? "connected" : "disconnected"}
        >
          <Badge 
            variant="outline"
            className={`font-medium px-3 py-1 ${
              connected 
                ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30" 
                : "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30"
            }`}
          >
            <span className={`inline-block w-2 h-2 rounded-full mr-2 ${
              connected ? "bg-emerald-500 status-dot-running" : "bg-red-500"
            }`} />
            {connected ? (
              <span className="flex items-center gap-1.5">
                <Wifi className="h-3 w-3" />
                Connected
              </span>
            ) : (
              <span className="flex items-center gap-1.5">
                <WifiOff className="h-3 w-3" />
                Disconnected
              </span>
            )}
          </Badge>
        </motion.div>

        {/* Session Info */}
        {sessionId && (
          <motion.div 
            initial={{ opacity: 0, x: -10 }}
            animate={{ opacity: 1, x: 0 }}
            className="flex items-center gap-2 text-sm"
          >
            <Terminal className="h-3.5 w-3.5 text-muted-foreground" />
            <span className="text-muted-foreground">Session:</span>
            <code className="px-2 py-0.5 bg-muted/50 rounded text-xs font-mono border border-border/50">
              {sessionId.substring(0, 8)}
            </code>
          </motion.div>
        )}

        {/* Pod Info */}
        {podName && (
          <motion.div 
            initial={{ opacity: 0, x: -10 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: 0.1 }}
            className="flex items-center gap-2 text-sm"
          >
            <Box className="h-3.5 w-3.5 text-blue-500" />
            <span className="text-muted-foreground">Pod:</span>
            <code className="px-2 py-0.5 bg-blue-500/10 text-blue-600 dark:text-blue-400 rounded text-xs font-mono border border-blue-500/30">
              {podName}
            </code>
          </motion.div>
        )}
      </div>

      {/* Actions */}
      <div className="flex gap-2">
        {!connected && onReconnect && (
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
          >
            <Button 
              size="sm" 
              onClick={onReconnect}
              className="bg-gradient-to-r from-blue-500 to-blue-600 hover:from-blue-600 hover:to-blue-700 text-white shadow-lg shadow-blue-500/25"
            >
              <RefreshCw className="h-3.5 w-3.5 mr-1.5" />
              Reconnect
            </Button>
          </motion.div>
        )}
        {connected && onDisconnect && (
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
          >
            <Button 
              size="sm" 
              variant="outline" 
              onClick={onDisconnect}
              className="hover:bg-red-500/10 hover:text-red-500 hover:border-red-500/30"
            >
              <Power className="h-3.5 w-3.5 mr-1.5" />
              Disconnect
            </Button>
          </motion.div>
        )}
      </div>
    </div>
  );
}
