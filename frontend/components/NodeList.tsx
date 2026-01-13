"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Loader2, RefreshCw, Server, Cpu, HardDrive, Network, ChevronDown, ChevronRight } from "lucide-react";

interface Node {
  name: string;
  status: string;
  ready: boolean;
  role: string;
  internalIP: string;
  externalIP: string;
  uptime: string;
  kubeletVersion: string;
  osImage: string;
  containerRuntime: string;
  architecture: string;
  operatingSystem: string;
  cpuCapacity: string;
  memoryCapacity: string;
  cpuAllocatable: string;
  memoryAllocatable: string;
  labels: Record<string, string>;
  taints: string[];
  age: string;
}

interface NodeListProps {
  onClose?: () => void;
}

export function NodeList({ onClose }: NodeListProps) {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());

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

  const fetchNodes = async () => {
    setLoading(true);
    setError(null);
    try {
      const apiUrl = getApiUrl();
      const url = `${apiUrl}/api/v1/nodes`;
      console.log("[NodeList] Fetching nodes from:", url);
      const response = await fetch(url);
      if (!response.ok) {
        const errorText = await response.text();
        console.error("[NodeList] API error:", response.status, errorText);
        setError(`Failed to fetch nodes: ${response.statusText}`);
        setNodes([]);
        return;
      }
      const data = await response.json();
      console.log("[NodeList] Received nodes:", data.nodes?.length || 0);
      // Log first node to debug role field
      if (data.nodes && data.nodes.length > 0) {
        console.log("[NodeList] First node sample:", {
          name: data.nodes[0].name,
          role: data.nodes[0].role,
          status: data.nodes[0].status
        });
      }
      setNodes(data.nodes || []);
    } catch (err) {
      console.error("[NodeList] Error fetching nodes:", err);
      if (err instanceof TypeError && err.message.includes("fetch")) {
        setError("Failed to connect to backend server. Please ensure the backend is running.");
      } else {
        setError(err instanceof Error ? err.message : "Failed to fetch nodes");
      }
      setNodes([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchNodes();
    // Refresh every 10 seconds
    const interval = setInterval(fetchNodes, 10000);
    return () => clearInterval(interval);
  }, []);

  const getStatusColor = (status: string, ready: boolean) => {
    if (status === "Ready" && ready) {
      return "bg-green-500/20 text-green-700 dark:text-green-400 border-green-500/50";
    }
    if (status === "NotReady") {
      return "bg-red-500/20 text-red-700 dark:text-red-400 border-red-500/50";
    }
    return "bg-yellow-500/20 text-yellow-700 dark:text-yellow-400 border-yellow-500/50";
  };

  const toggleNode = (nodeName: string) => {
    setExpandedNodes((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(nodeName)) {
        newSet.delete(nodeName);
      } else {
        newSet.add(nodeName);
      }
      return newSet;
    });
  };

  const getRoleColor = (role: string) => {
    if (role === "control-plane" || role === "master") {
      return "bg-purple-500/20 text-purple-700 dark:text-purple-400 border-purple-500/50";
    }
    return "bg-blue-500/20 text-blue-700 dark:text-blue-400 border-blue-500/50";
  };

  return (
    <Card className="p-4 h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Server className="h-5 w-5" />
            Kubernetes Nodes
          </h2>
          <p className="text-sm text-muted-foreground">
            {nodes.length} node{nodes.length !== 1 ? "s" : ""} found
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={fetchNodes}
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

      {loading && nodes.length === 0 ? (
        <div className="flex items-center justify-center flex-1">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : error ? (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : nodes.length === 0 ? (
        <div className="flex items-center justify-center flex-1 text-muted-foreground">
          No nodes found
        </div>
      ) : (
        <div className="flex-1 overflow-auto">
          <div className="space-y-2">
            {nodes.map((node) => {
              const isExpanded = expandedNodes.has(node.name);
              return (
                <Card
                  key={node.name}
                  className="border-2 transition-colors"
                >
                  {/* Collapsed Header - Always Visible */}
                  <div
                    className="p-3 hover:bg-accent/50 cursor-pointer flex items-center justify-between gap-2"
                    onClick={() => toggleNode(node.name)}
                  >
                    <div className="flex items-center gap-2 flex-1 min-w-0">
                      {isExpanded ? (
                        <ChevronDown className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                      )}
                      <h3 className="font-semibold truncate">{node.name}</h3>
                      <Badge
                        variant="outline"
                        className={getStatusColor(node.status, node.ready)}
                      >
                        {node.status}
                      </Badge>
                      {node.role && node.role.trim() !== "" ? (
                        <Badge
                          variant="outline"
                          className={getRoleColor(node.role)}
                        >
                          {node.role}
                        </Badge>
                      ) : (
                        <Badge
                          variant="outline"
                          className={getRoleColor("worker")}
                        >
                          worker
                        </Badge>
                      )}
                    </div>
                  </div>

                  {/* Expanded Content */}
                  {isExpanded && (
                    <div className="p-4 pt-0 space-y-3 border-t">

                      {/* Network Info */}
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                        <div className="space-y-2">
                          <div className="flex items-center gap-2 text-sm">
                            <Network className="h-4 w-4 text-muted-foreground" />
                            <span className="font-medium">Network:</span>
                          </div>
                          <div className="pl-6 space-y-1 text-sm text-muted-foreground">
                            {node.internalIP && (
                              <div>
                                <span className="font-medium">Internal IP:</span>{" "}
                                <span className="font-mono text-blue-600 dark:text-blue-400">
                                  {node.internalIP}
                                </span>
                              </div>
                            )}
                            {node.externalIP && (
                              <div>
                                <span className="font-medium">External IP:</span>{" "}
                                <span className="font-mono text-purple-600 dark:text-purple-400">
                                  {node.externalIP}
                                </span>
                              </div>
                            )}
                          </div>
                        </div>

                        {/* Resources */}
                        <div className="space-y-2">
                          <div className="flex items-center gap-2 text-sm">
                            <Cpu className="h-4 w-4 text-muted-foreground" />
                            <span className="font-medium">Resources:</span>
                          </div>
                          <div className="pl-6 space-y-1 text-sm text-muted-foreground">
                            <div>
                              <span className="font-medium">CPU:</span>{" "}
                              <span className="font-mono text-green-600 dark:text-green-400">
                                {node.cpuAllocatable} / {node.cpuCapacity}
                              </span>
                            </div>
                            <div>
                              <span className="font-medium">Memory:</span>{" "}
                              <span className="font-mono text-green-600 dark:text-green-400">
                                {node.memoryAllocatable} / {node.memoryCapacity}
                              </span>
                            </div>
                          </div>
                        </div>
                      </div>

                      {/* System Info */}
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 pt-2 border-t">
                        <div className="space-y-1 text-sm">
                          <div className="text-muted-foreground">
                            <span className="font-medium">OS:</span> {node.operatingSystem} ({node.architecture})
                          </div>
                          <div className="text-muted-foreground">
                            <span className="font-medium">OS Image:</span> {node.osImage}
                          </div>
                          <div className="text-muted-foreground">
                            <span className="font-medium">Container Runtime:</span> {node.containerRuntime}
                          </div>
                        </div>
                        <div className="space-y-1 text-sm">
                          <div className="text-muted-foreground">
                            <span className="font-medium">Kubelet Version:</span>{" "}
                            <span className="font-mono text-blue-600 dark:text-blue-400">
                              {node.kubeletVersion}
                            </span>
                          </div>
                          <div className="text-muted-foreground">
                            <span className="font-medium">Uptime:</span>{" "}
                            <span className="text-green-600 dark:text-green-400 font-medium">
                              {node.uptime}
                            </span>
                          </div>
                          <div className="text-muted-foreground">
                            <span className="font-medium">Age:</span> {node.age}
                          </div>
                        </div>
                      </div>

                      {/* Taints */}
                      {node.taints && node.taints.length > 0 && (
                        <div className="pt-2 border-t">
                          <div className="text-sm font-medium mb-1">Taints:</div>
                          <div className="flex flex-wrap gap-1">
                            {node.taints.map((taint, idx) => (
                              <Badge
                                key={idx}
                                variant="outline"
                                className="bg-yellow-500/20 text-yellow-700 dark:text-yellow-400 border-yellow-500/50 text-xs"
                              >
                                {taint}
                              </Badge>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Labels */}
                      {node.labels && Object.keys(node.labels).length > 0 && (
                        <div className="pt-2 border-t">
                          <div className="text-sm font-medium mb-1">Labels:</div>
                          <div className="flex flex-wrap gap-1">
                            {Object.entries(node.labels).slice(0, 5).map(([key, value]) => (
                              <Badge
                                key={key}
                                variant="outline"
                                className="text-xs"
                              >
                                {key}={value}
                              </Badge>
                            ))}
                            {Object.keys(node.labels).length > 5 && (
                              <Badge variant="outline" className="text-xs">
                                +{Object.keys(node.labels).length - 5} more
                              </Badge>
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </Card>
              );
            })}
          </div>
        </div>
      )}
    </Card>
  );
}
