"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Loader2, Trash2, RefreshCw, FileText } from "lucide-react";
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

export function PodList({ namespace = "default", onClose }: PodListProps) {
  const [pods, setPods] = useState<Pod[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<Set<string>>(new Set());
  const [logsPod, setLogsPod] = useState<{ name: string; namespace: string } | null>(null);

  const getApiUrl = () => {
    if (process.env.NEXT_PUBLIC_API_URL) {
      const url = process.env.NEXT_PUBLIC_API_URL;
      // If it already has a protocol, use it as-is
      if (url.startsWith("http://") || url.startsWith("https://")) {
        return url;
      }
      // Otherwise, assume http and add protocol
      return `http://${url}`;
    }
    // Default to same origin or localhost
    if (typeof window !== "undefined") {
      return `${window.location.protocol}//${window.location.hostname}:8080`;
    }
    return "http://localhost:8080";
  };

  const fetchPods = async () => {
    setLoading(true);
    setError(null);
    try {
      const apiUrl = getApiUrl();
      const url = `${apiUrl}/api/v1/pods?namespace=${encodeURIComponent(namespace)}`;
      console.log("[PodList] Fetching pods from:", url, "namespace:", namespace);
      const response = await fetch(url);
      if (!response.ok) {
        const errorText = await response.text();
        console.error("[PodList] API error:", response.status, errorText);
        
        // Check if it's a 404 or invalid namespace error
        if (response.status === 404 || response.status === 403) {
          setError(`Namespace "${namespace}" not found or access denied. Please verify the namespace exists and you have permissions to access it.`);
        } else {
          setError(`Failed to fetch pods: ${response.statusText}`);
        }
        setPods([]);
        return;
      }
      const data = await response.json();
      console.log("[PodList] Received pods:", data.pods?.length || 0);
      setPods(data.pods || []);
      
      // If no pods found, check if namespace might be invalid
      if (data.pods && data.pods.length === 0) {
        // This is fine - namespace exists but has no pods
        console.log("[PodList] No pods found in namespace:", namespace);
      }
    } catch (err) {
      console.error("[PodList] Error fetching pods:", err);
      if (err instanceof TypeError && err.message.includes("fetch")) {
        setError("Failed to connect to backend server. Please ensure the backend is running.");
      } else {
        setError(err instanceof Error ? err.message : "Failed to fetch pods");
      }
      setPods([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPods();
    // Refresh every 5 seconds
    const interval = setInterval(fetchPods, 5000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [namespace]);

  const handleDelete = async (podName: string, podNamespace?: string) => {
    const targetNamespace = podNamespace || namespace;
    if (!confirm(`Are you sure you want to delete pod "${podName}" in namespace "${targetNamespace}"?`)) {
      return;
    }

    setDeleting((prev) => new Set(prev).add(podName));
    try {
      const apiUrl = getApiUrl();
      const response = await fetch(
        `${apiUrl}/api/v1/pods/${podName}?namespace=${encodeURIComponent(targetNamespace)}`,
        {
          method: "DELETE",
        }
      );
      if (!response.ok) {
        throw new Error(`Failed to delete pod: ${response.statusText}`);
      }
      // Refresh the list
      await fetchPods();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to delete pod");
    } finally {
      setDeleting((prev) => {
        const next = new Set(prev);
        next.delete(podName);
        return next;
      });
    }
  };

  const getStatusColor = (status: string, ready: boolean) => {
    if (status === "Running" && ready) {
      return "bg-green-500/20 text-green-700 dark:text-green-400 border-green-500/50";
    }
    if (status === "Running") {
      return "bg-yellow-500/20 text-yellow-700 dark:text-yellow-400 border-yellow-500/50";
    }
    if (status === "Pending") {
      return "bg-blue-500/20 text-blue-700 dark:text-blue-400 border-blue-500/50";
    }
    if (status === "Failed" || status === "Error") {
      return "bg-red-500/20 text-red-700 dark:text-red-400 border-red-500/50";
    }
    return "bg-gray-500/20 text-gray-700 dark:text-gray-400 border-gray-500/50";
  };

  return (
    <Card className="p-4 h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold">Pods in {namespace}</h2>
          <p className="text-sm text-muted-foreground">
            {pods.length} pod{pods.length !== 1 ? "s" : ""} found
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={fetchPods}
            disabled={loading}
          >
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
          </Button>
          {onClose && (
            <Button variant="outline" size="sm" onClick={onClose}>
              Close
            </Button>
          )}
        </div>
      </div>

      {loading && pods.length === 0 ? (
        <div className="flex items-center justify-center flex-1">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : error ? (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : pods.length === 0 ? (
        <div className="flex items-center justify-center flex-1 text-muted-foreground">
          No pods found in namespace &quot;{namespace}&quot;
        </div>
      ) : (
        <div className="flex-1 overflow-auto">
          <div className="space-y-2">
            {pods.map((pod) => (
              <Card
                key={`${pod.namespace}-${pod.name}`}
                className="p-3 hover:bg-accent/50 transition-colors"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      <h3 className="font-medium truncate">{pod.name}</h3>
                      <Badge
                        variant="outline"
                        className={getStatusColor(pod.status, pod.ready)}
                      >
                        {pod.status}
                      </Badge>
                      {pod.ready && (
                        <Badge variant="outline" className="bg-green-500/20 text-green-700 dark:text-green-400 border-green-500/50">
                          Ready
                        </Badge>
                      )}
                    </div>
                    <div className="text-sm text-muted-foreground space-y-1">
                      <div>Namespace: {pod.namespace}</div>
                      <div>Age: {pod.age}</div>
                      {pod.restarts > 0 && (
                        <div className="text-yellow-600 dark:text-yellow-500">
                          Restarts: {pod.restarts}
                        </div>
                      )}
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setLogsPod({ name: pod.name, namespace: pod.namespace })}
                      title="View logs"
                    >
                      <FileText className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => handleDelete(pod.name, pod.namespace)}
                      disabled={deleting.has(pod.name)}
                      title="Delete pod"
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
            ))}
          </div>
        </div>
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
