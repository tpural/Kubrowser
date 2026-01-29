"use client";

import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import toast from "react-hot-toast";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import {
  RefreshCw,
  Server,
  Cpu,
  HardDrive,
  Network,
  ChevronDown,
  ChevronRight,
  Clock,
  Container,
  Activity,
  Zap,
  MemoryStick,
} from "lucide-react";

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
  cpuUsage: string;
  memoryUsage: string;
  labels: Record<string, string>;
  taints: string[];
  age: string;
}

interface NodeListProps {
  onClose?: () => void;
}

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.08,
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
};

const expandVariants = {
  hidden: { opacity: 0, height: 0 },
  visible: {
    opacity: 1,
    height: "auto",
    transition: {
      height: { duration: 0.3 },
      opacity: { duration: 0.2, delay: 0.1 },
    },
  },
  exit: {
    opacity: 0,
    height: 0,
    transition: {
      height: { duration: 0.2 },
      opacity: { duration: 0.1 },
    },
  },
};

// Resource bar component
function ResourceBar({
  used,
  total,
  label,
  color = "blue",
}: {
  used: string;
  total: string;
  label: string;
  color?: "blue" | "green" | "purple" | "amber";
}) {
  // Parse memory values (e.g., "16Gi" -> 16)
  // Parse memory values (e.g., "16Gi" -> 16)
  const parseValue = (val: string | undefined | null): number => {
    if (!val) return 0;
    const num = parseFloat(val.replace(/[^0-9.]/g, ""));
    return isNaN(num) ? 0 : num;
  };

  const usedNum = parseValue(used);
  const totalNum = parseValue(total);
  const percentage =
    totalNum > 0 ? Math.min((usedNum / totalNum) * 100, 100) : 0;

  const colorClasses = {
    blue: "from-blue-500 to-cyan-400",
    green: "from-emerald-500 to-teal-400",
    purple: "from-purple-500 to-pink-400",
    amber: "from-amber-500 to-orange-400",
  };

  return (
    <div className="space-y-1.5">
      <div className="flex justify-between text-xs">
        <span className="text-muted-foreground font-medium">{label}</span>
        <span className="font-mono text-foreground">
          {used} / {total}
        </span>
      </div>
      <div className="resource-bar h-2">
        <motion.div
          className={`resource-bar-fill bg-gradient-to-r ${colorClasses[color]}`}
          initial={{ width: 0 }}
          animate={{ width: `${percentage}%` }}
          transition={{ duration: 0.8, ease: "easeOut" }}
        />
      </div>
    </div>
  );
}

export function NodeList({ onClose }: NodeListProps) {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());
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

  const fetchNodes = async (showToast = false) => {
    setLoading(true);
    setError(null);
    try {
      const apiUrl = getApiUrl();
      const url = `${apiUrl}/api/v1/nodes`;
      const response = await fetch(url, { credentials: "include" });
      if (!response.ok) {
        setError(`Failed to fetch nodes: ${response.statusText}`);
        setNodes([]);
        return;
      }
      const data = await response.json();
      setNodes(data.nodes || []);
      if (showToast) {
        toast.success(`Refreshed ${data.nodes?.length || 0} nodes`);
      }
    } catch (err) {
      console.error("[NodeList] Error fetching nodes:", err);
      if (err instanceof TypeError && err.message.includes("fetch")) {
        setError("Failed to connect to backend server.");
      } else {
        setError(err instanceof Error ? err.message : "Failed to fetch nodes");
      }
      setNodes([]);
    } finally {
      setLoading(false);
      setInitialLoad(false);
    }
  };

  useEffect(() => {
    fetchNodes();
    const interval = setInterval(() => fetchNodes(false), 10000);
    return () => clearInterval(interval);
  }, []);

  const getStatusDot = (status: string, ready: boolean) => {
    if (status === "Ready" && ready) return "status-dot status-dot-running";
    if (status === "NotReady") return "status-dot status-dot-failed";
    return "status-dot status-dot-pending";
  };

  const getStatusColor = (status: string, ready: boolean) => {
    if (status === "Ready" && ready) {
      return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30";
    }
    if (status === "NotReady") {
      return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30";
    }
    return "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30";
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
      return "bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/30";
    }
    return "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30";
  };

  const getRoleIcon = (role: string) => {
    if (role === "control-plane" || role === "master") {
      return <Zap className="h-3 w-3 mr-1" />;
    }
    return <Activity className="h-3 w-3 mr-1" />;
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
          {[...Array(3)].map((_, i) => (
            <Card key={i} className="p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Skeleton className="h-5 w-5" />
                  <Skeleton className="h-5 w-48" />
                  <Skeleton className="h-5 w-16" />
                  <Skeleton className="h-5 w-24" />
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
            <div className="p-1.5 rounded-lg bg-gradient-to-br from-emerald-500 to-teal-600">
              <Server className="h-4 w-4 text-white" />
            </div>
            <span>Kubernetes Nodes</span>
          </h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            {nodes.length} node{nodes.length !== 1 ? "s" : ""} •
            {nodes.filter((n) => n.ready).length} ready
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => fetchNodes(true)}
            disabled={loading}
            className="relative overflow-hidden group"
          >
            <RefreshCw
              className={`h-4 w-4 transition-transform group-hover:rotate-180 duration-500 ${loading ? "animate-spin" : ""}`}
            />
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
          <Alert
            variant="destructive"
            className="border-red-500/50 bg-red-500/10"
          >
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </motion.div>
      ) : nodes.length === 0 ? (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="flex flex-col items-center justify-center flex-1 text-muted-foreground py-12"
        >
          <Server className="h-12 w-12 mb-4 opacity-20" />
          <p className="text-lg font-medium">No nodes found</p>
        </motion.div>
      ) : (
        <motion.div
          variants={containerVariants}
          initial="hidden"
          animate="visible"
          className="flex-1 overflow-auto custom-scrollbar pr-1"
        >
          <div className="space-y-2">
            {nodes.map((node) => {
              const isExpanded = expandedNodes.has(node.name);
              return (
                <motion.div key={node.name} variants={itemVariants} layout>
                  <Card
                    className={`card-hover overflow-hidden transition-all duration-300 ${
                      isExpanded ? "ring-2 ring-blue-500/20" : ""
                    }`}
                  >
                    {/* Header - Always Visible */}
                    <motion.div
                      className="p-3 cursor-pointer flex items-center justify-between gap-2 hover:bg-accent/30 transition-colors"
                      onClick={() => toggleNode(node.name)}
                      whileTap={{ scale: 0.995 }}
                    >
                      <div className="flex items-center gap-3 flex-1 min-w-0">
                        <motion.div
                          animate={{ rotate: isExpanded ? 90 : 0 }}
                          transition={{ duration: 0.2 }}
                        >
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
                        </motion.div>
                        <span
                          className={getStatusDot(node.status, node.ready)}
                        />
                        <h3 className="font-semibold truncate">{node.name}</h3>
                        <Badge
                          variant="outline"
                          className={`${getStatusColor(node.status, node.ready)} text-xs`}
                        >
                          {node.status}
                        </Badge>
                        <Badge
                          variant="outline"
                          className={`${getRoleColor(node.role || "worker")} text-xs flex items-center`}
                        >
                          {getRoleIcon(node.role || "worker")}
                          {node.role || "worker"}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <Clock className="h-3 w-3" />
                        {node.uptime}
                      </div>
                    </motion.div>

                    {/* Expanded Content */}
                    <AnimatePresence>
                      {isExpanded && (
                        <motion.div
                          variants={expandVariants}
                          initial="hidden"
                          animate="visible"
                          exit="exit"
                          className="overflow-hidden"
                        >
                          <div className="p-4 pt-0 space-y-4 border-t bg-accent/5">
                            {/* Resource Bars */}
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-4">
                              <div className="space-y-3">
                                <div className="flex items-center gap-2 text-sm font-medium">
                                  <Cpu className="h-4 w-4 text-blue-500" />
                                  <span>CPU</span>
                                </div>
                                <ResourceBar
                                  used={node.cpuUsage || "0"}
                                  total={node.cpuCapacity}
                                  label="Usage"
                                  color="blue"
                                />
                              </div>
                              <div className="space-y-3">
                                <div className="flex items-center gap-2 text-sm font-medium">
                                  <MemoryStick className="h-4 w-4 text-purple-500" />
                                  <span>Memory</span>
                                </div>
                                <ResourceBar
                                  used={node.memoryUsage || "0"}
                                  total={node.memoryCapacity}
                                  label="Usage"
                                  color="purple"
                                />
                              </div>
                            </div>

                            {/* Network & System Info */}
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2">
                              {/* Network */}
                              <div className="space-y-2 p-3 rounded-lg bg-card/50">
                                <div className="flex items-center gap-2 text-sm font-medium">
                                  <Network className="h-4 w-4 text-cyan-500" />
                                  <span>Network</span>
                                </div>
                                <div className="space-y-1 text-sm">
                                  {node.internalIP && (
                                    <div className="flex justify-between">
                                      <span className="text-muted-foreground">
                                        Internal IP
                                      </span>
                                      <span className="font-mono text-cyan-600 dark:text-cyan-400">
                                        {node.internalIP}
                                      </span>
                                    </div>
                                  )}
                                  {node.externalIP && (
                                    <div className="flex justify-between">
                                      <span className="text-muted-foreground">
                                        External IP
                                      </span>
                                      <span className="font-mono text-purple-600 dark:text-purple-400">
                                        {node.externalIP}
                                      </span>
                                    </div>
                                  )}
                                </div>
                              </div>

                              {/* System */}
                              <div className="space-y-2 p-3 rounded-lg bg-card/50">
                                <div className="flex items-center gap-2 text-sm font-medium">
                                  <Container className="h-4 w-4 text-amber-500" />
                                  <span>System</span>
                                </div>
                                <div className="space-y-1 text-sm">
                                  <div className="flex justify-between">
                                    <span className="text-muted-foreground">
                                      OS
                                    </span>
                                    <span>
                                      {node.operatingSystem} (
                                      {node.architecture})
                                    </span>
                                  </div>
                                  <div className="flex justify-between">
                                    <span className="text-muted-foreground">
                                      Kubelet
                                    </span>
                                    <span className="font-mono text-blue-600 dark:text-blue-400">
                                      {node.kubeletVersion}
                                    </span>
                                  </div>
                                </div>
                              </div>
                            </div>

                            {/* Additional Info */}
                            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2 text-sm">
                              <div className="p-3 rounded-lg bg-card/50">
                                <div className="text-muted-foreground mb-1">
                                  OS Image
                                </div>
                                <div className="truncate" title={node.osImage}>
                                  {node.osImage}
                                </div>
                              </div>
                              <div className="p-3 rounded-lg bg-card/50">
                                <div className="text-muted-foreground mb-1">
                                  Container Runtime
                                </div>
                                <div className="truncate">
                                  {node.containerRuntime}
                                </div>
                              </div>
                              <div className="p-3 rounded-lg bg-card/50">
                                <div className="text-muted-foreground mb-1">
                                  Age
                                </div>
                                <div>{node.age}</div>
                              </div>
                            </div>

                            {/* Taints */}
                            {node.taints && node.taints.length > 0 && (
                              <div className="pt-2">
                                <div className="text-sm font-medium mb-2 flex items-center gap-2">
                                  <HardDrive className="h-4 w-4 text-amber-500" />
                                  Taints
                                </div>
                                <div className="flex flex-wrap gap-1.5">
                                  {node.taints.map((taint, idx) => (
                                    <Badge
                                      key={idx}
                                      variant="outline"
                                      className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30 text-xs"
                                    >
                                      {taint}
                                    </Badge>
                                  ))}
                                </div>
                              </div>
                            )}

                            {/* Labels */}
                            {node.labels &&
                              Object.keys(node.labels).length > 0 && (
                                <div className="pt-2">
                                  <div className="text-sm font-medium mb-2">
                                    Labels
                                  </div>
                                  <div className="flex flex-wrap gap-1.5">
                                    {Object.entries(node.labels)
                                      .slice(0, 6)
                                      .map(([key, value]) => (
                                        <Badge
                                          key={key}
                                          variant="outline"
                                          className="text-xs bg-card/80"
                                        >
                                          {key}={value}
                                        </Badge>
                                      ))}
                                    {Object.keys(node.labels).length > 6 && (
                                      <Badge
                                        variant="secondary"
                                        className="text-xs"
                                      >
                                        +{Object.keys(node.labels).length - 6}{" "}
                                        more
                                      </Badge>
                                    )}
                                  </div>
                                </div>
                              )}
                          </div>
                        </motion.div>
                      )}
                    </AnimatePresence>
                  </Card>
                </motion.div>
              );
            })}
          </div>
        </motion.div>
      )}
    </Card>
  );
}
