package cluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/teko/food-delivery/internal/model"
)

const (
	serviceAccountPath = "/var/run/secrets/kubernetes.io/serviceaccount/"
	maxResponseBytes   = 2 << 20
)

// Observer reads a deliberately small, read-only part of the Kubernetes API.
type Observer struct {
	baseURL   string
	namespace string
	token     string
	client    *http.Client
}

type workloadList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Replicas int `json:"replicas"`
		} `json:"spec"`
		Status struct {
			ReadyReplicas int `json:"readyReplicas"`
		} `json:"status"`
	} `json:"items"`
}

type clusterList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Instances int `json:"instances"`
		} `json:"spec"`
		Status struct {
			ReadyInstances int    `json:"readyInstances"`
			CurrentPrimary string `json:"currentPrimary"`
		} `json:"status"`
	} `json:"items"`
}

// NewInCluster creates an observer from the mounted ServiceAccount credentials.
func NewInCluster(namespace string) (*Observer, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("Kubernetes service environment is missing")
	}
	token, err := os.ReadFile(serviceAccountPath + "token")
	if err != nil {
		return nil, fmt.Errorf("read ServiceAccount token: %w", err)
	}
	caPEM, err := os.ReadFile(serviceAccountPath + "ca.crt")
	if err != nil {
		return nil, fmt.Errorf("read Kubernetes CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse Kubernetes CA")
	}
	return &Observer{
		baseURL:   "https://" + host + ":" + port,
		namespace: namespace,
		token:     strings.TrimSpace(string(token)),
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    roots,
			}},
		},
	}, nil
}

// Components returns current application workload readiness.
func (o *Observer) Components(ctx context.Context) ([]model.Component, error) {
	components := make([]model.Component, 0, 16)
	deployments := workloadList{}
	if err := o.get(ctx, fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments", o.namespace), &deployments); err != nil {
		return nil, err
	}
	for _, item := range deployments.Items {
		components = append(components, component(item.Metadata.Name, "Deployment", item.Status.ReadyReplicas, item.Spec.Replicas))
	}

	statefulSets := workloadList{}
	if err := o.get(ctx, fmt.Sprintf("/apis/apps/v1/namespaces/%s/statefulsets", o.namespace), &statefulSets); err != nil {
		return nil, err
	}
	for _, item := range statefulSets.Items {
		components = append(components, component(item.Metadata.Name, "StatefulSet", item.Status.ReadyReplicas, item.Spec.Replicas))
	}

	clusters := clusterList{}
	path := fmt.Sprintf("/apis/postgresql.cnpg.io/v1/namespaces/%s/clusters", o.namespace)
	if err := o.get(ctx, path, &clusters); err == nil {
		for _, item := range clusters.Items {
			status := "degraded"
			if item.Status.ReadyInstances == item.Spec.Instances && item.Spec.Instances > 0 {
				status = "healthy"
			}
			components = append(components, model.Component{
				ID: item.Metadata.Name, Name: "PostgreSQL", Kind: "CNPG Cluster", Status: status,
				Ready: item.Status.ReadyInstances, Desired: item.Spec.Instances, Detail: "Primary: " + item.Status.CurrentPrimary, Category: "data",
			})
		}
	}
	sort.Slice(components, func(i, j int) bool {
		if components[i].Category == components[j].Category {
			return components[i].Name < components[j].Name
		}
		return components[i].Category < components[j].Category
	})
	return components, nil
}

func (o *Observer) get(ctx context.Context, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create Kubernetes request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+o.token)
	response, err := o.client.Do(request)
	if err != nil {
		return fmt.Errorf("call Kubernetes API: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("Kubernetes API %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes)).Decode(target); err != nil {
		return fmt.Errorf("decode Kubernetes response: %w", err)
	}
	return nil
}

func component(name, kind string, ready, desired int) model.Component {
	status := "degraded"
	if ready == desired && desired > 0 {
		status = "healthy"
	}
	category := "service"
	detail := "application workload"
	displayName := strings.NewReplacer("-", " ").Replace(name)
	if name == "dashboard" {
		category, detail = "edge", "Nuxt + PixiJS"
	}
	if name == "rabbitmq" {
		category, detail = "platform", "food.events"
	}
	if strings.HasPrefix(name, "restaurant-") {
		detail = "restaurant kitchen pods"
		return model.Component{ID: name, Name: displayName, Kind: kind, Status: status, Ready: ready, Desired: desired, Detail: detail, Category: category, EntityKind: "restaurant", EntityID: name}
	}
	if name == "customer-simulator" {
		detail = "persistent customer agents"
		return model.Component{ID: name, Name: displayName, Kind: kind, Status: status, Ready: ready, Desired: desired, Detail: detail, Category: category, EntityKind: "customer"}
	}
	if name == "courier-simulator" {
		detail = "persistent courier fleet"
		return model.Component{ID: name, Name: displayName, Kind: kind, Status: status, Ready: ready, Desired: desired, Detail: detail, Category: category, EntityKind: "courier"}
	}
	return model.Component{ID: name, Name: displayName, Kind: kind, Status: status, Ready: ready, Desired: desired, Detail: detail, Category: category}
}
