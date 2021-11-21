package main

import (
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodRole string

type ReplicaList map[PodRole]int64

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GenericResource is the Schema for any resource supported by resource-metrics library
type GenericResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GenericResourceSpec  `json:"spec,omitempty"`
	Status runtime.RawExtension `json:"status,omitempty"`
}

type GenericResourceSpec struct {
	Group   string
	Version string
	Kind    string

	Replicas     int64
	RoleReplicas ReplicaList
	Mode         string

	TotalResource core.ResourceRequirements

	// TotalResourceLimits core.ResourceList
	// TotalResourceRequests core.ResourceList

	AppResource core.ResourceRequirements

	// AppResourceLimits core.ResourceList
	// AppResourceRequests core.ResourceList

	RoleResourceLimits   map[PodRole]core.ResourceList
	RoleResourceRequests map[PodRole]core.ResourceList

	// https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus
	Status string // kstatus
}
