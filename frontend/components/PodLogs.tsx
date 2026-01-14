"use client";

import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Loader2, RefreshCw, FileText, Download, ArrowDown } from "lucide-react";

interface PodLogsProps {
  podName: string;
  namespace: string;
  onClose: () => void;
}

export function PodLogs({ podName, namespace, onClose }: PodLogsProps) {
  const [logs, setLogs] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const logsContainerRef = useRef<HTMLDivElement>(null);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const readerRef = useRef<ReadableStreamDefaultReader<Uint8Array> | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const mountedRef = useRef(true);

  const getApiUrl = () => {
    if (process.env.NEXT_PUBLIC_API_URL) {
      const url = process.env.NEXT_PUBLIC_API_URL;
      if (url.startsWith("http://") || url.startsWith("https://")) {
        return url;
      }
      return `http://${url}`;
    }
    if (typeof window !== "undefined") {
      return `${window.location.protocol}//${window.location.hostname}:8080`;
    }
    return "http://localhost:8080";
  };

  const fetchLogs = () => {
    try {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      if (readerRef.current) {
        readerRef.current.cancel().catch(() => {});
      }
    } catch {
      // Ignore
    }

    setLoading(true);
    setError(null);
    setLogs("");
    
    const apiUrl = getApiUrl();
    const url = `${apiUrl}/api/v1/pods/${podName}/logs?namespace=${encodeURIComponent(namespace)}&tail=500&follow=true`;
    
    const abortController = new AbortController();
    abortControllerRef.current = abortController;
    
    fetch(url, { signal: abortController.signal })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Failed to fetch logs: ${response.statusText}`);
        }
        setLoading(false);
        
        const reader = response.body?.getReader();
        const decoder = new TextDecoder();
        
        if (!reader) {
          throw new Error("No response body");
        }
        
        readerRef.current = reader;
        
        const readStream = () => {
          reader.read().then(({ done, value }) => {
            if (!mountedRef.current) return;
            if (done) return;
            const chunk = decoder.decode(value, { stream: true });
            setLogs((prev) => prev + chunk);
            readStream();
          }).catch((err) => {
            if (!mountedRef.current) return;
            if (err.name !== "AbortError" && err.name !== "NetworkError") {
              setError(err.message);
              setLoading(false);
            }
          });
        };
        
        readStream();
      })
      .catch((err) => {
        if (!mountedRef.current) return;
        if (err.name !== "AbortError") {
          setError(err instanceof Error ? err.message : "Failed to fetch logs");
          setLoading(false);
        }
      });
  };

  const downloadLogs = () => {
    const blob = new Blob([logs], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${podName}-${namespace}-logs.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const scrollToBottom = () => {
    if (logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: "smooth" });
    }
  };

  useEffect(() => {
    mountedRef.current = true;
    fetchLogs();
    
    return () => {
      mountedRef.current = false;
      try {
        if (abortControllerRef.current) {
          abortControllerRef.current.abort();
          abortControllerRef.current = null;
        }
      } catch {}
      try {
        if (readerRef.current) {
          readerRef.current.cancel().catch(() => {});
          readerRef.current = null;
        }
      } catch {}
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [podName, namespace]);

  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs, autoScroll]);

  // Detect manual scroll
  const handleScroll = () => {
    if (!logsContainerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = logsContainerRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
    setAutoScroll(isAtBottom);
  };

  const lineCount = logs.split('\n').length - 1;

  return (
    <Dialog open={true} onOpenChange={(open) => !open && onClose()}>
      <DialogContent 
        className="max-h-[90vh] flex flex-col p-0 gap-0 overflow-hidden"
        style={{ maxWidth: '70vw', width: '70vw' }}
      >
        {/* Header */}
        <DialogHeader className="px-6 py-4 border-b bg-gradient-to-r from-emerald-500/10 to-teal-500/10">
          <DialogTitle className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-emerald-500/20">
              <FileText className="h-4 w-4 text-emerald-500" />
            </div>
            <div>
              <div className="font-semibold">{podName}</div>
              <div className="text-xs text-muted-foreground font-normal">
                Namespace: {namespace} • {lineCount} lines
              </div>
            </div>
          </DialogTitle>
        </DialogHeader>

        {/* Toolbar */}
        <div className="flex justify-between items-center px-4 py-2 border-b bg-muted/30">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            {loading && (
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                className="flex items-center gap-1.5 text-emerald-500"
              >
                <div className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                Streaming...
              </motion.div>
            )}
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={downloadLogs}
              disabled={!logs}
              className="h-8 text-xs"
            >
              <Download className="h-3.5 w-3.5 mr-1.5" />
              Download
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={fetchLogs}
              disabled={loading}
              className="h-8 text-xs"
            >
              <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${loading ? "animate-spin" : ""}`} />
              Refresh
            </Button>
          </div>
        </div>

        {/* Logs Content */}
        <div className="flex-1 h-[60vh] overflow-hidden relative">
          <div 
            ref={logsContainerRef}
            onScroll={handleScroll}
            className="h-full overflow-auto bg-[#0d1117] text-[#c9d1d9] font-mono text-xs p-4 custom-scrollbar"
          >
            <AnimatePresence mode="wait">
              {loading && !logs ? (
                <motion.div
                  key="loading"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  className="flex items-center justify-center h-full"
                >
                  <div className="flex flex-col items-center gap-3">
                    <Loader2 className="h-8 w-8 animate-spin text-emerald-500" />
                    <span className="text-sm text-muted-foreground">Loading logs...</span>
                  </div>
                </motion.div>
              ) : error ? (
                <motion.div
                  key="error"
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  className="flex flex-col items-center justify-center h-full gap-2"
                >
                  <div className="text-red-400 text-sm">{error}</div>
                  <Button variant="outline" size="sm" onClick={fetchLogs}>
                    Try again
                  </Button>
                </motion.div>
              ) : logs ? (
                <motion.div
                  key="logs"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                >
                  <pre className="whitespace-pre-wrap break-words leading-relaxed">
                    {logs.split('\n').map((line, i) => (
                      <div 
                        key={i} 
                        className="hover:bg-white/5 -mx-4 px-4 py-0.5 flex"
                      >
                        <span className="text-[#6e7681] select-none w-12 flex-shrink-0 text-right pr-4">
                          {i + 1}
                        </span>
                        <span className={
                          line.toLowerCase().includes('error') ? 'text-red-400' :
                          line.toLowerCase().includes('warn') ? 'text-amber-400' :
                          line.toLowerCase().includes('info') ? 'text-blue-400' :
                          ''
                        }>
                          {line}
                        </span>
                      </div>
                    ))}
                  </pre>
                </motion.div>
              ) : (
                <motion.div
                  key="empty"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="flex items-center justify-center h-full text-muted-foreground"
                >
                  No logs available
                </motion.div>
              )}
            </AnimatePresence>
            <div ref={logsEndRef} />
          </div>

          {/* Scroll to bottom button */}
          <AnimatePresence>
            {!autoScroll && logs && (
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 10 }}
                className="absolute bottom-4 right-6"
              >
                <Button
                  size="sm"
                  onClick={scrollToBottom}
                  className="shadow-lg bg-emerald-600 hover:bg-emerald-700 text-white"
                >
                  <ArrowDown className="h-4 w-4 mr-1.5" />
                  Scroll to bottom
                </Button>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </DialogContent>
    </Dialog>
  );
}
