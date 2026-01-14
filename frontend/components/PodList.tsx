"use client";

import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import toast from "react-hot-toast";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { Loader2, Trash2, RefreshCw, FileText, Box, Clock, RotateCcw, Layers } from "lucide-react";
import { PodLogs } from "./PodLogs";

interface Pod {
  name: string;
  namespace: string;
  status: string;
  ready: boolean;
  age: string;
  restarts: number;
}

interface PodListProps {
  namespace?: string;
  onClose?: () => void;
}

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20, scale: 0.95 },
  visible: {
    opacity: 1,
    y: 0,
    scale: 1,
    transition: {
      type: "spring" as const,
      stiffness: 300,
      damping: 24,
    },
  },
  exit: {
    opacity: 0,
    scale: 0.95,
    x: -20,
    transition: { duration: 0.2 },
  },
};

export function PodList({ namespace = "default", onClose }: PodListProps) {
  const [pods, setPods] = useState<Pod[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<Set<string>>(new Set());
  const [logsPod, setLogsPod] = useState<{ name: string; namespace: string } | null>(null);
  const [initialLoad, setInitialLoad] = useState(true);

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

  const fetchPods = async (showToast = false) => {
    setLoading(true);
    setError(null);
    try {
      const apiUrl = getApiUrl();
      const url = `${apiUrl}/api/v1/pods?namespace=${encodeURIComponent(namespace)}`;
      const response = await fetch(url);
      if (!response.ok) {
        if (response.status === 404 || response.status === 403) {
          setError(`Namespace "${namespace}" not found or access denied.`);
        } else {
          setError(`Failed to fetch pods: ${response.statusText}`);
        }
        setPods([]);
        return;
      }
      const data = await response.json();
      setPods(data.pods || []);
      if (showToast) {
        toast.success(`Refreshed ${data.pods?.length || 0} pods`);
      }
    } catch (err) {
      console.error("[PodList] Error fetching pods:", err);
      if (err instanceof TypeError && err.message.includes("fetch")) {
        setError("Failed to connect to backend server.");
      } else {
        setError(err instanceof Error ? err.message : "Failed to fetch pods");
      }
      setPods([]);
    } finally {
      setLoading(false);
      setInitialLoad(false);
    }
  };

  useEffect(() => {
    fetchPods();
    const interval = setInterval(() => fetchPods(false), 5000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [namespace]);

  const handleDelete = async (podName: string, podNamespace?: string) => {
    const targetNamespace = podNamespace || namespace;
    if (!confirm(`Delete pod "${podName}"?`)) {
      return;
    }

    setDeleting((prev) => new Set(prev).add(podName));
    const deletePromise = (async () => {
      const apiUrl = getApiUrl();
      const response = await fetch(
        `${apiUrl}/api/v1/pods/${podName}?namespace=${encodeURIComponent(targetNamespace)}`,
        { method: "DELETE" }
      );
      if (!response.ok) {
        throw new Error(`Failed to delete pod: ${response.statusText}`);
      }
      await fetchPods();
      return podName;
    })();

    toast.promise(deletePromise, {
      loading: `Deleting ${podName}...`,
      success: (name) => `Pod ${name} deleted`,
      error: (err) => err.message,
    });

    try {
      await deletePromise;
    } finally {
      setDeleting((prev) => {
        const next = new Set(prev);
        next.delete(podName);
        return next;
      });
    }
  };

  const getStatusDot = (status: string, ready: boolean) => {
    if (status === "Running" && ready) return "status-dot status-dot-running";
    if (status === "Running" || status === "Pending") return "status-dot status-dot-pending";
    if (status === "Failed" || status === "Error") return "status-dot status-dot-failed";
    return "status-dot status-dot-unknown";
  };

  const getStatusColor = (status: string, ready: boolean) => {
    if (status === "Running" && ready) {
      return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30";
    }
    if (status === "Running") {
      return "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30";
    }
    if (status === "Pending") {
      return "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30";
    }
    if (status === "Failed" || status === "Error") {
      return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30";
    }
    return "bg-gray-500/10 text-gray-600 dark:text-gray-400 border-gray-500/30";
  };

  // Skeleton loading
  if (initialLoad) {
    return (
      <Card className="p-4 h-full flex flex-col glass">
        <div className="flex items-center justify-between mb-4">
          <div className="space-y-2">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-24" />
          </div>
          <div className="flex gap-2">
            <Skeleton className="h-9 w-9" />
            <Skeleton className="h-9 w-16" />
          </div>
        </div>
        <div className="space-y-3 flex-1">
          {[...Array(4)].map((_, i) => (
            <Card key={i} className="p-4">
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 space-y-3">
                  <div className="flex items-center gap-2">
                    <Skeleton className="h-5 w-48" />
                    <Skeleton className="h-5 w-16" />
                  </div>
                  <div className="space-y-2">
                    <Skeleton className="h-4 w-32" />
                    <Skeleton className="h-4 w-24" />
                  </div>
                </div>
                <div className="flex gap-2">
                  <Skeleton className="h-8 w-8" />
                  <Skeleton className="h-8 w-8" />
                </div>
              </div>
            </Card>
          ))}
        </div>
      </Card>
    );
  }

  return (
    <Card className="p-4 h-full flex flex-col glass overflow-hidden">
      {/* Header */}
      <motion.div 
        initial={{ opacity: 0, y: -10 }}
        animate={{ opacity: 1, y: 0 }}
        className="flex items-center justify-between mb-4"
      >
        <div>
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600">
              <Layers className="h-4 w-4 text-white" />
            </div>
            <span>Pods in <span className="text-blue-500">{namespace}</span></span>
          </h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            {pods.length} pod{pods.length !== 1 ? "s" : ""} found
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => fetchPods(true)}
            disabled={loading}
            className="relative overflow-hidden group"
          >
            <RefreshCw className={`h-4 w-4 transition-transform group-hover:rotate-180 duration-500 ${loading ? "animate-spin" : ""}`} />
          </Button>
          {onClose && (
            <Button variant="outline" size="sm" onClick={onClose}>
              Close
            </Button>
          )}
        </div>
      </motion.div>

      {/* Content */}
      {error ? (
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
        >
          <Alert variant="destructive" className="border-red-500/50 bg-red-500/10">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </motion.div>
      ) : pods.length === 0 ? (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="flex flex-col items-center justify-center flex-1 text-muted-foreground py-12"
        >
          <Box className="h-12 w-12 mb-4 opacity-20" />
          <p className="text-lg font-medium">No pods found</p>
          <p className="text-sm">in namespace &quot;{namespace}&quot;</p>
        </motion.div>
      ) : (
        <motion.div
          variants={containerVariants}
          initial="hidden"
          animate="visible"
          className="flex-1 overflow-auto custom-scrollbar pr-1"
        >
          <AnimatePresence mode="popLayout">
            <div className="space-y-2">
              {pods.map((pod, index) => (
                <motion.div
                  key={`${pod.namespace}-${pod.name}`}
                  variants={itemVariants}
                  layout
                  exit="exit"
                  style={{ animationDelay: `${index * 50}ms` }}
                >
                  <Card className="p-3 card-hover border-l-4 border-l-transparent hover:border-l-blue-500 transition-all duration-300 bg-card/50 backdrop-blur-sm">
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-2">
                          <span className={getStatusDot(pod.status, pod.ready)} />
                          <h3 className="font-medium truncate text-sm">{pod.name}</h3>
                          <Badge
                            variant="outline"
                            className={`${getStatusColor(pod.status, pod.ready)} text-xs font-medium`}
                          >
                            {pod.status}
                          </Badge>
                          {pod.ready && (
                            <Badge 
                              variant="outline" 
                              className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 text-xs"
                            >
                              ✓ Ready
                            </Badge>
                          )}
                        </div>
                        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                          <span className="flex items-center gap-1">
                            <Clock className="h-3 w-3" />
                            {pod.age}
                          </span>
                          {pod.restarts > 0 && (
                            <span className="flex items-center gap-1 text-amber-600 dark:text-amber-400">
                              <RotateCcw className="h-3 w-3" />
                              {pod.restarts} restart{pod.restarts !== 1 ? "s" : ""}
                            </span>
                          )}
                        </div>
                      </div>
                      <div className="flex gap-1.5">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setLogsPod({ name: pod.name, namespace: pod.namespace })}
                          title="View logs"
                          className="h-8 w-8 p-0 hover:bg-blue-500/10 hover:text-blue-500"
                        >
                          <FileText className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDelete(pod.name, pod.namespace)}
                          disabled={deleting.has(pod.name)}
                          title="Delete pod"
                          className="h-8 w-8 p-0 hover:bg-red-500/10 hover:text-red-500"
                        >
                          {deleting.has(pod.name) ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Trash2 className="h-4 w-4" />
                          )}
                        </Button>
                      </div>
                    </div>
                  </Card>
                </motion.div>
              ))}
            </div>
          </AnimatePresence>
        </motion.div>
      )}
      
      {/* Logs Dialog */}
      {logsPod && (
        <PodLogs
          podName={logsPod.name}
          namespace={logsPod.namespace}
          onClose={() => setLogsPod(null)}
        />
      )}
    </Card>
  );
}
