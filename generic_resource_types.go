package main

import (
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/resource-metrics/api"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

type PodRole string

type ReplicaList map[PodRole]int64

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GenericResource is the Schema for any resource supported by resource-metrics library
type GenericResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GenericResourceSpec `json:"spec,omitempty"`
	Status status.Result       `json:"status,omitempty"`
}

type GenericResourceSpec struct {
	Group   string
	Version string
	Kind    string

	Replicas     int64
	RoleReplicas api.ReplicaList
	Mode         string

	TotalResource core.ResourceRequirements

	// TotalResourceLimits core.ResourceList
	// TotalResourceRequests core.ResourceList

	AppResource core.ResourceRequirements

	// AppResourceLimits core.ResourceList
	// AppResourceRequests core.ResourceList

	RoleResourceLimits   map[api.PodRole]core.ResourceList
	RoleResourceRequests map[api.PodRole]core.ResourceList

	// https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus
	// Status string // kstatus
}
