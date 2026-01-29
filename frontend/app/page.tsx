"use client";

import { useState, useRef } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Terminal, TerminalHandle } from "@/components/Terminal";
import { StatusBar } from "@/components/StatusBar";
import { UserMenu } from "@/components/UserMenu";
import { PodList } from "@/components/PodList";
import { NodeList } from "@/components/NodeList";
import { Card } from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { AlertCircle, RefreshCw, Pin, PinOff, Sparkles } from "lucide-react";

const slideVariants = {
  hidden: { opacity: 0, x: 50, scale: 0.95 },
  visible: {
    opacity: 1,
    x: 0,
    scale: 1,
    transition: {
      type: "spring" as const,
      stiffness: 300,
      damping: 30,
    },
  },
  exit: {
    opacity: 0,
    x: 50,
    scale: 0.95,
    transition: { duration: 0.2 },
  },
};

import { useTerminalSession } from "@/hooks/useTerminalSession";
import { useAuth } from "@/hooks/useAuth";
import { SplashScreen } from "@/components/SplashScreen";

export default function Home() {
  const {
    connected,
    sessionId,
    podName,
    shouldDisconnect,
    error,
    handleConnect,
    handleDisconnect,
    handleReconnect,
    handleError,
    clearError
  } = useTerminalSession();

  const [showPodList, setShowPodList] = useState(false);
  const [podListNamespace, setPodListNamespace] = useState<string>("default");
  const [showNodeList, setShowNodeList] = useState(false);
  const [isPinned, setIsPinned] = useState(false);
  const terminalRef = useRef<TerminalHandle>(null);

  const { user, loading, login } = useAuth();

  const handleCommandDetected = (command: string, namespace?: string) => {
    if (isPinned) {
      return;
    }

    if (command === "kubectl get pods") {
      const detectedNamespace = namespace || "default";
      setPodListNamespace(detectedNamespace);
      setShowPodList(true);
      setShowNodeList(false);
    } else if (command === "kubectl get nodes") {
      setShowNodeList(true);
      setShowPodList(false);
    }
  };

  const handleCommandClose = () => {
    if (isPinned) {
      return;
    }
    setShowPodList(false);
    setShowNodeList(false);
  };

  const handlePodListClose = () => {
    setShowPodList(false);
    setIsPinned(false);
    if (terminalRef.current) {
      terminalRef.current.resetDetection();
    }
  };

  const handleNodeListClose = () => {
    setShowNodeList(false);
    setIsPinned(false);
    if (terminalRef.current) {
      terminalRef.current.resetDetection();
    }
  };

  const togglePin = () => {
    setIsPinned(!isPinned);
  };

  const hasPopout = showPodList || showNodeList;

  return (
    <div className="flex h-screen flex-col bg-gradient-to-br from-background via-background to-muted/30">
      {/* Header */}
      <motion.header
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        className="border-b bg-card/80 backdrop-blur-xl supports-[backdrop-filter]:bg-card/60 sticky top-0 z-50"
      >
        <div className="flex items-center justify-between py-3 px-4">
          <div className="flex items-center gap-4">
            <motion.div
              className="relative h-14 w-14 flex-shrink-0"
              whileHover={{ scale: 1.05, rotate: 5 }}
              transition={{ type: "spring", stiffness: 400 }}
            >
              <img
                src="/logo.svg"
                alt="Kubrowser Logo"
                className="h-full w-full object-contain drop-shadow-lg"
              />
            </motion.div>
            <div>
              <h1
                className="text-2xl font-bold tracking-tight bg-gradient-to-r from-[#326CE5] to-[#5B8FF9] bg-clip-text text-transparent"
                style={{ fontFamily: 'var(--font-jetbrains-mono)' }}
              >
                Kubrowser
              </h1>
              <p className="text-xs text-muted-foreground flex items-center gap-1.5">
                <Sparkles className="h-3 w-3" />
                Browser-based kubectl terminal
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 0.2 }}
            >
              <UserMenu />
            </motion.div>
          </div>
        </div>
      </motion.header>

      {/* Main Content */}
      <div className="flex-1 overflow-hidden p-4">
        {loading || !user ? (
          <SplashScreen onLogin={login} loading={loading} />
        ) : (
          <div className="h-full flex gap-4">
            {/* Terminal Panel */}
            <motion.div
              layout
              transition={{ type: "spring", stiffness: 300, damping: 30 }}
              className={`${hasPopout ? "w-1/2" : "w-full"} flex flex-col`}
            >
              <Card className="flex-1 flex flex-col shadow-xl border-2 overflow-hidden bg-card/50 backdrop-blur-sm">
                {/* Terminal Header */}
                <div className="p-3 border-b bg-gradient-to-r from-muted/50 to-muted/30">
                  <StatusBar
                    connected={connected}
                    sessionId={sessionId}
                    podName={podName}
                    onReconnect={handleReconnect}
                    onDisconnect={handleDisconnect}
                  />
                </div>

                {/* Terminal Content */}
                <div className="flex-1 overflow-hidden p-4 bg-background relative">
                  {/* Error Overlay */}
                  <AnimatePresence>
                    {error && (
                      <motion.div
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="absolute inset-0 z-20 flex items-center justify-center bg-black/90 backdrop-blur-sm p-4"
                      >
                        <motion.div
                          initial={{ scale: 0.9, y: 20 }}
                          animate={{ scale: 1, y: 0 }}
                          exit={{ scale: 0.9, y: 20 }}
                        >
                          <Alert variant="destructive" className="max-w-2xl border-red-500/50 bg-red-950/50">
                            <AlertCircle className="h-4 w-4" />
                            <AlertTitle>Connection Error</AlertTitle>
                            <AlertDescription className="mt-2 space-y-3">
                              <p>{error}</p>
                              <div className="flex gap-2 pt-2">
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={handleReconnect}
                                  className="border-red-500/50 hover:bg-red-500/10"
                                >
                                  <RefreshCw className="h-4 w-4 mr-2" />
                                  Retry Connection
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={clearError}
                                >
                                  Dismiss
                                </Button>
                              </div>
                            </AlertDescription>
                          </Alert>
                        </motion.div>
                      </motion.div>
                    )}
                  </AnimatePresence>

                  <Terminal
                    ref={terminalRef}
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
            </motion.div>

            {/* Popout Panels */}
            <AnimatePresence mode="wait">
              {showPodList && (
                <motion.div
                  key="podlist"
                  variants={slideVariants}
                  initial="hidden"
                  animate="visible"
                  exit="exit"
                  className="w-1/2 flex flex-col"
                >
                  <Card className={`flex-1 flex flex-col shadow-xl border-2 overflow-hidden transition-colors duration-300 ${isPinned ? "border-amber-500/50 ring-2 ring-amber-500/20" : ""
                    }`}>
                    {/* Panel Header */}
                    <div className="flex items-center justify-between p-3 border-b bg-gradient-to-r from-blue-500/10 to-purple-500/10">
                      <div className="flex items-center gap-2">
                        <div className="h-2 w-2 rounded-full bg-blue-500 animate-pulse" />
                        <span className="text-sm font-semibold">
                          Pods — <span className="text-blue-500">{podListNamespace}</span>
                        </span>
                      </div>
                      <motion.div whileHover={{ scale: 1.05 }} whileTap={{ scale: 0.95 }}>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={togglePin}
                          className={`h-8 w-8 p-0 ${isPinned ? "text-amber-500 bg-amber-500/10" : "hover:bg-muted"}`}
                          title={isPinned ? "Unpin" : "Pin"}
                        >
                          {isPinned ? <PinOff className="h-4 w-4" /> : <Pin className="h-4 w-4" />}
                        </Button>
                      </motion.div>
                    </div>
                    <div className="flex-1 overflow-hidden">
                      <PodList
                        key={podListNamespace}
                        namespace={podListNamespace}
                        onClose={handlePodListClose}
                      />
                    </div>
                  </Card>
                </motion.div>
              )}

              {showNodeList && (
                <motion.div
                  key="nodelist"
                  variants={slideVariants}
                  initial="hidden"
                  animate="visible"
                  exit="exit"
                  className="w-1/2 flex flex-col"
                >
                  <Card className={`flex-1 flex flex-col shadow-xl border-2 overflow-hidden transition-colors duration-300 ${isPinned ? "border-amber-500/50 ring-2 ring-amber-500/20" : ""
                    }`}>
                    {/* Panel Header */}
                    <div className="flex items-center justify-between p-3 border-b bg-gradient-to-r from-emerald-500/10 to-teal-500/10">
                      <div className="flex items-center gap-2">
                        <div className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                        <span className="text-sm font-semibold">Kubernetes Nodes</span>
                      </div>
                      <motion.div whileHover={{ scale: 1.05 }} whileTap={{ scale: 0.95 }}>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={togglePin}
                          className={`h-8 w-8 p-0 ${isPinned ? "text-amber-500 bg-amber-500/10" : "hover:bg-muted"}`}
                          title={isPinned ? "Unpin" : "Pin"}
                        >
                          {isPinned ? <PinOff className="h-4 w-4" /> : <Pin className="h-4 w-4" />}
                        </Button>
                      </motion.div>
                    </div>
                    <div className="flex-1 overflow-hidden">
                      <NodeList onClose={handleNodeListClose} />
                    </div>
                  </Card>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        )}
      </div>
    </div>
  );
}
