"use client";

import { useState } from "react";
import { Terminal } from "@/components/Terminal";
import { StatusBar } from "@/components/StatusBar";
import { ThemeToggle } from "@/components/ThemeToggle";
import { PodList } from "@/components/PodList";
import { Card } from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { AlertCircle, RefreshCw } from "lucide-react";

export default function Home() {
  const [connected, setConnected] = useState(false);
  const [sessionId, setSessionId] = useState<string | undefined>();
  const [podName] = useState<string | undefined>();
  const [shouldDisconnect, setShouldDisconnect] = useState(false);
  const [showPodList, setShowPodList] = useState(false);
  const [podListNamespace, setPodListNamespace] = useState<string>("default");
  const [error, setError] = useState<string | null>(null);

  const handleConnect = (newSessionId: string) => {
    setSessionId(newSessionId);
    setConnected(true);
    setShouldDisconnect(false); // Reset disconnect flag when reconnecting
    setError(null); // Clear any previous errors on successful connection
  };

  const handleDisconnect = () => {
    setShouldDisconnect(true);
    setConnected(false);
  };

  const handleReconnect = () => {
    // Terminal component will handle reconnection
    setError(null);
    window.location.reload();
  };

  const handleError = (errorMessage: string) => {
    setError(errorMessage);
    setConnected(false);
  };

  const handleCommandDetected = (command: string, namespace?: string) => {
    if (command === "kubectl get pods") {
      // Use provided namespace or default to "default"
      const detectedNamespace = namespace || "default";
      console.log("Command detected:", command, "Namespace:", detectedNamespace);
      setPodListNamespace(detectedNamespace);
      setShowPodList(true);
    }
  };

  const handleCommandClose = () => {
    console.log("Command close detected - closing pod list");
    setShowPodList(false);
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
        <div className="h-full flex gap-4">
          <Card className={`${showPodList ? "w-1/2" : "w-full"} flex flex-col shadow-lg border-2 transition-all duration-300`}>
            <div className="p-3 border-b bg-muted/30">
              <StatusBar
                connected={connected}
                sessionId={sessionId}
                podName={podName}
                onReconnect={handleReconnect}
                onDisconnect={handleDisconnect}
              />
            </div>
            <div className="flex-1 overflow-hidden p-4 bg-background flex flex-col relative">
              {error && (
                <div className="absolute inset-0 z-20 flex items-center justify-center bg-background/95 backdrop-blur-sm p-4">
                  <Alert variant="destructive" className="max-w-2xl">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Connection Error</AlertTitle>
                    <AlertDescription className="mt-2 space-y-3">
                      <p>{error}</p>
                      <div className="flex gap-2 pt-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={handleReconnect}
                        >
                          <RefreshCw className="h-4 w-4 mr-2" />
                          Retry Connection
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setError(null)}
                        >
                          Dismiss
                        </Button>
                      </div>
                    </AlertDescription>
                  </Alert>
                </div>
              )}
              <Terminal
                sessionId={sessionId}
                onConnect={handleConnect}
                onDisconnect={handleDisconnect}
                onError={handleError}
                shouldDisconnect={shouldDisconnect}
                onCommandDetected={handleCommandDetected}
                onCommandClose={handleCommandClose}
              />
            </div>
          </Card>
          {showPodList && (
            <Card className="w-1/2 flex flex-col shadow-lg border-2">
              <div className="flex-1 overflow-hidden p-4">
                <PodList
                  key={podListNamespace} // Force re-mount when namespace changes
                  namespace={podListNamespace}
                  onClose={() => setShowPodList(false)}
                />
              </div>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}
