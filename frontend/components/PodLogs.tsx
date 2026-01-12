"use client";

import { useEffect, useRef, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Loader2, RefreshCw } from "lucide-react";

interface PodLogsProps {
  podName: string;
  namespace: string;
  onClose: () => void;
}

export function PodLogs({ podName, namespace, onClose }: PodLogsProps) {
  const [logs, setLogs] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
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
    // Cancel previous request if any (silently)
    try {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      if (readerRef.current) {
        readerRef.current.cancel().catch(() => {
          // Ignore cancel errors
        });
      }
    } catch {
      // Ignore errors from canceling
    }

    setLoading(true);
    setError(null);
    setLogs("");
    
    const apiUrl = getApiUrl();
    const url = `${apiUrl}/api/v1/pods/${podName}/logs?namespace=${encodeURIComponent(namespace)}&tail=500&follow=true`;
    
    const abortController = new AbortController();
    abortControllerRef.current = abortController;
    
    // Use fetch with streaming
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
            if (!mountedRef.current) return; // Don't update if unmounted
            if (done) {
              return;
            }
            const chunk = decoder.decode(value, { stream: true });
            setLogs((prev) => prev + chunk);
            readStream();
          }).catch((err) => {
            // Ignore AbortError - it's expected when refreshing or unmounting
            if (!mountedRef.current) return; // Don't update if unmounted
            if (err.name !== "AbortError" && err.name !== "NetworkError") {
              setError(err.message);
              setLoading(false);
            }
          });
        };
        
        readStream();
      })
      .catch((err) => {
        // Ignore AbortError - it's expected when refreshing or unmounting
        if (!mountedRef.current) return; // Don't update if unmounted
        if (err.name !== "AbortError") {
          setError(err instanceof Error ? err.message : "Failed to fetch logs");
          setLoading(false);
        }
      });
  };

  useEffect(() => {
    mountedRef.current = true;
    fetchLogs();
    
    return () => {
      mountedRef.current = false; // Mark as unmounted first
      
      // Cleanup on unmount - silently abort/cancel to avoid errors
      try {
        if (abortControllerRef.current) {
          abortControllerRef.current.abort();
          abortControllerRef.current = null;
        }
      } catch (err) {
        // Ignore abort errors during cleanup - they're expected
      }
      
      try {
        if (readerRef.current) {
          readerRef.current.cancel().catch(() => {
            // Ignore cancel errors - expected during cleanup
          });
          readerRef.current = null;
        }
      } catch (err) {
        // Ignore any errors during reader cancellation
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [podName, namespace]);

  useEffect(() => {
    // Auto-scroll to bottom when logs update
    if (logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs]);

  return (
    <Dialog open={true} onOpenChange={(open) => !open && onClose()}>
      <DialogContent 
        className="max-h-[90vh] flex flex-col"
        style={{ maxWidth: '65vw', width: '65vw' }}
      >
        <DialogHeader>
          <DialogTitle>
            Logs: {podName} ({namespace})
          </DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-hidden flex flex-col gap-2">
          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={fetchLogs}
              disabled={loading}
            >
              <RefreshCw className={`h-4 w-4 mr-2 ${loading ? "animate-spin" : ""}`} />
              Refresh
            </Button>
          </div>
          <div className="flex-1 overflow-auto bg-black text-green-400 font-mono text-sm p-4 rounded border">
            {loading ? (
              <div className="flex items-center justify-center h-full">
                <Loader2 className="h-6 w-6 animate-spin" />
              </div>
            ) : error ? (
              <div className="text-red-400">{error}</div>
            ) : logs ? (
              <pre className="whitespace-pre-wrap break-words">{logs}</pre>
            ) : (
              <div className="text-gray-500">No logs available</div>
            )}
            <div ref={logsEndRef} />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
