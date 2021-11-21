package main

import (
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
)

type ResourceSummarySpec struct {
	Kubernetes    *KubernetesInfo `json:"kubernetes"`
	APIGroup      string
	Kind          string
	TotalResource core.ResourceRequirements
	AppResource   core.ResourceRequirements
	Count         int
}

type KubernetesInfo struct {
	// https://github.com/kmodules/client-go/blob/master/tools/clusterid/lib.go
	ClusterName  string            `json:"clusterName,omitempty"`
	ClusterUID   string            `json:"clusterUID,omitempty"`
	Version      *version.Info     `json:"version,omitempty"`
	ControlPlane *ControlPlaneInfo `json:"controlPlane,omitempty"`
}

// https://github.com/kmodules/client-go/blob/kubernetes-1.16.3/tools/analytics/analytics.go#L66
type ControlPlaneInfo struct {
	DNSNames       []string    `json:"dnsNames,omitempty"`
	EmailAddresses []string    `json:"emailAddresses,omitempty"`
	IPAddresses    []string    `json:"ipAddresses,omitempty"`
	URIs           []string    `json:"uris,omitempty"`
	NotBefore      metav1.Time `json:"notBefore"`
	NotAfter       metav1.Time `json:"notAfter"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ResourceSummary is the Schema for the clustersummaries API
type ResourceSummary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ResourceSummarySpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ResourceSummaryList contains a list of ResourceSummary
type ResourceSummaryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceSummary `json:"items"`
}
