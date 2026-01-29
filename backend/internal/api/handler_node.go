package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// NodeMetrics structs for parsing metrics.k8s.io response.
type NodeMetricsList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Usage struct {
			CPU    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"usage"`
	} `json:"items"`
}

// HandleListNodes lists all Kubernetes nodes.
func (h *Handlers) HandleListNodes(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	nodes, err := h.podManager.GetClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		h.logger.WithError(err).Error("Failed to list nodes")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list nodes"})
		return
	}

	h.logger.WithField("nodes_count", len(nodes.Items)).Info("Successfully listed nodes")

	// Fetch metrics.
	metricsMap := make(map[string]map[string]string)

	// Create a REST client for metrics.
	config := h.podManager.GetConfig()
	if config != nil {
		metricsConfig := *config
		metricsConfig.GroupVersion = &schema.GroupVersion{Group: "metrics.k8s.io", Version: "v1beta1"}
		metricsConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
		metricsConfig.APIPath = "/apis"

		var metricsClient *rest.RESTClient
		metricsClient, err = rest.RESTClientFor(&metricsConfig)
		if err == nil {
			var data []byte
			data, err = metricsClient.Get().Resource("nodes").DoRaw(ctx)
			if err == nil {
				var metricsList NodeMetricsList
				if err = json.Unmarshal(data, &metricsList); err == nil {
					for _, item := range metricsList.Items {
						// Parse CPU.
						cpuStr := "0"
						if cpuQ, cpuErr := resource.ParseQuantity(item.Usage.CPU); cpuErr == nil {
							// Convert to cores (float).
							cpuStr = fmt.Sprintf("%.2f", float64(cpuQ.MilliValue())/1000.0)
						}

						// Parse Memory.
						memStr := "0"
						if memQ, memErr := resource.ParseQuantity(item.Usage.Memory); memErr == nil {
							// Convert to GB.
							memGB := float64(memQ.Value()) / (1024 * 1024 * 1024)
							memStr = fmt.Sprintf("%.2f GB", memGB)
						}

						metricsMap[item.Metadata.Name] = map[string]string{
							"cpu":    cpuStr,
							"memory": memStr,
						}
					}
				} else {
					h.logger.WithError(err).Warn("Failed to unmarshal node metrics")
				}
			} else {
				h.logger.WithError(err).Warn("Failed to fetch node metrics")
			}
		} else {
			h.logger.WithError(err).Warn("Failed to create metrics client")
		}
	}

	nodeList := make([]gin.H, 0, len(nodes.Items))
	for i := range nodes.Items {
		node := &nodes.Items[i]
		// Get node status.
		ready := false
		status := "NotReady"
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				if condition.Status == v1.ConditionTrue {
					ready = true
					status = "Ready"
				} else {
					status = "NotReady"
				}
				break
			}
		}

		// Get internal IP.
		internalIP := ""
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP {
				internalIP = addr.Address
				break
			}
		}

		// Get external IP (if available).
		externalIP := ""
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeExternalIP {
				externalIP = addr.Address
				break
			}
		}

		// Calculate uptime from creation timestamp and format as days/hours.
		uptimeDuration := time.Since(node.CreationTimestamp.Time)
		days := int(uptimeDuration.Hours() / 24)
		hours := int(uptimeDuration.Hours()) % 24
		var uptimeStr string
		if days > 0 {
			uptimeStr = fmt.Sprintf("%dd %dh", days, hours)
		} else {
			uptimeStr = fmt.Sprintf("%dh", hours)
		}

		// The presence of the label itself indicates the role.
		role := "worker"
		if node.Labels != nil {
			// Label exists (value can be empty or "true").
			if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
				role = "control-plane"
			} else if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
				// Check for master role (older Kubernetes versions).
				role = "master"
			} else if val, ok := node.Labels["kubernetes.io/role"]; ok && val != "" {
				// Check for generic role label.
				role = val
			} else {
				// Check all labels for any node-role pattern.
				for key := range node.Labels {
					if strings.HasPrefix(key, "node-role.kubernetes.io/") {
						parts := strings.Split(key, "/")
						if len(parts) > 1 {
							// Extract role from label key (e.g., "control-plane" from "node-role.kubernetes.io/control-plane").
							roleName := parts[1]
							if roleName != "" {
								role = roleName
								break
							}
						}
					}
				}
			}
		}

		// Get node info.
		kubeletVersion := node.Status.NodeInfo.KubeletVersion
		osImage := node.Status.NodeInfo.OSImage
		containerRuntime := node.Status.NodeInfo.ContainerRuntimeVersion
		architecture := node.Status.NodeInfo.Architecture
		operatingSystem := node.Status.NodeInfo.OperatingSystem

		// Get CPU and memory capacity.
		cpuCapacity := node.Status.Capacity[v1.ResourceCPU]
		memoryCapacity := node.Status.Capacity[v1.ResourceMemory]
		cpuAllocatable := node.Status.Allocatable[v1.ResourceCPU]
		memoryAllocatable := node.Status.Allocatable[v1.ResourceMemory]

		// Convert memory to GB.
		memoryCapacityGB := float64(memoryCapacity.Value()) / (1024 * 1024 * 1024)
		memoryAllocatableGB := float64(memoryAllocatable.Value()) / (1024 * 1024 * 1024)

		// Get Usage if available.
		cpuUsage := "0"
		memoryUsage := "0"
		if metrics, ok := metricsMap[node.Name]; ok {
			cpuUsage = metrics["cpu"]
			memoryUsage = metrics["memory"]
		}

		// Get labels.
		labels := make(map[string]string)
		if node.Labels != nil {
			for k, v := range node.Labels {
				labels[k] = v
			}
		}

		// Get taints.
		taints := make([]string, 0)
		for _, taint := range node.Spec.Taints {
			taints = append(taints, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}

		// Ensure role is never empty.
		if role == "" {
			role = "worker"
		}

		nodeList = append(nodeList, gin.H{
			"name":              node.Name,
			"status":            status,
			"ready":             ready,
			"role":              role,
			"internalIP":        internalIP,
			"externalIP":        externalIP,
			"uptime":            uptimeStr,
			"kubeletVersion":    kubeletVersion,
			"osImage":           osImage,
			"containerRuntime":  containerRuntime,
			"architecture":      architecture,
			"operatingSystem":   operatingSystem,
			"cpuCapacity":       cpuCapacity.String(),
			"memoryCapacity":    fmt.Sprintf("%.2f GB", memoryCapacityGB),
			"cpuAllocatable":    cpuAllocatable.String(),
			"memoryAllocatable": fmt.Sprintf("%.2f GB", memoryAllocatableGB),
			"cpuUsage":          cpuUsage,
			"memoryUsage":       memoryUsage,
			"labels":            labels,
			"taints":            taints,
			"age":               time.Since(node.CreationTimestamp.Time).Round(time.Second).String(),
		})

		// Log role for debugging.
		h.logger.WithFields(logrus.Fields{
			"node": node.Name,
			"role": role,
		}).Debug("Node role extracted")
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodeList})
}
