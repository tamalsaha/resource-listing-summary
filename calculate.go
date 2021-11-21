package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	cu "kmodules.xyz/client-go/client"
	resourcemetrics "kmodules.xyz/resource-metrics"
	"kmodules.xyz/resource-metrics/api"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetAPIGroups(s labels.Selector) sets.String {
	g, found := s.RequiresExactMatch("k8s.io/group")
	if found {
		return sets.NewString(g)
	}

	requirements, selectable := s.Requirements()
	if selectable {
		for _, r := range requirements {
			if r.Key() == "k8s.io/group" && r.Operator() == selection.In {
				return r.Values()
			}
		}
	}
	return sets.NewString()
}

func calculate(c client.Client, ki *KubernetesInfo, apiGroups sets.String) error {
	clusterID, err := cu.ClusterUID(c)
	if err != nil {
		return err
	}

	rsList := make([]GenericResource, 0)
	// summaryList := make([]ResourceSummary, 0)

	rsmap := map[schema.GroupVersionKind]ResourceSummary{}
	var (
		totalCount int
		rrTotal    core.ResourceList
	)
	for _, gvk := range api.RegisteredTypes() {
		if apiGroups.Len() > 0 && !apiGroups.Has(gvk.Group) {
			continue
		}

		_, err := c.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if meta.IsNoMatchError(err) {
			rsmap[gvk] = ResourceSummary{} // keep track
			continue
		} else if err != nil {
			return err
		}

		var result unstructured.UnstructuredList
		result.SetGroupVersionKind(gvk)
		if err := c.List(context.TODO(), &result); err != nil {
			return err
		}

		summary := ResourceSummary{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      gvk.GroupKind().String(),
				Namespace: "",
			},
			Spec: ResourceSummarySpec{
				Kubernetes: ki,
				APIGroup:   gvk.Group,
				Kind:       gvk.Kind,
				// TotalResource: core.ResourceRequirements{},
				// AppResource:   core.ResourceRequirements{},
				Count: 0,
			},
		}

		for _, item := range result.Items {
			genres, err := ToGenericResource(item, gvk)
			if err != nil {
				return err
			}
			rsList = append(rsList, *genres)

			summary.Spec.TotalResource.Requests = api.AddResourceList(summary.Spec.TotalResource.Requests, genres.Spec.TotalResource.Requests)
			summary.Spec.TotalResource.Limits = api.AddResourceList(summary.Spec.TotalResource.Limits, genres.Spec.TotalResource.Limits)
			summary.Spec.AppResource.Requests = api.AddResourceList(summary.Spec.AppResource.Requests, genres.Spec.AppResource.Requests)
			summary.Spec.AppResource.Limits = api.AddResourceList(summary.Spec.AppResource.Limits, genres.Spec.AppResource.Limits)
		}
		summary.Spec.Count = len(result.Items)
		rsmap[gvk] = summary

		// global total
		totalCount += len(result.Items)
		rrTotal = api.AddResourceList(rrTotal, summary.Spec.AppResource.Limits)
	}

	gvks := make([]schema.GroupVersionKind, 0, len(rsmap))
	for gvk := range rsmap {
		gvks = append(gvks, gvk)
	}
	sort.Slice(gvks, func(i, j int) bool {
		if gvks[i].Group == gvks[j].Group {
			return gvks[i].Kind < gvks[j].Kind
		}
		return gvks[i].Group < gvks[j].Group
	})

	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.TabIndent)
	_, _ = fmt.Fprintln(os.Stdout, "")
	_, _ = fmt.Fprintf(os.Stdout, "CLUSTER ID: %s\n", clusterID)
	_, _ = fmt.Fprintln(os.Stdout, "")
	_, _ = fmt.Fprintln(w, "API VERSION\tKIND\tCOUNT\tCPU\tMEMORY\tSTORAGE\t")
	for _, gvk := range gvks {
		rr := rsmap[gvk]
		if rr.Spec.Count == 0 {
			_, _ = fmt.Fprintf(w, "%s\t%s\t-\t-\t-\t-\t\n", gvk.GroupVersion(), gvk.Kind)
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t\n", gvk.GroupVersion(), gvk.Kind, rr.Spec.Count, rr.Spec.AppResource.Limits.Cpu(), rr.Spec.AppResource.Limits.Memory(), rr.Spec.AppResource.Limits.Storage())
		}
	}
	_, _ = fmt.Fprintf(w, "TOTAL\t=\t%d\t%s\t%s\t%s\t\n", totalCount, rrTotal.Cpu(), rrTotal.Memory(), rrTotal.Storage())
	return w.Flush()
}

func ToGenericResource(item unstructured.Unstructured, gvk schema.GroupVersionKind) (*GenericResource, error) {
	content := item.UnstructuredContent()

	itemStatus, err := status.Compute(&item)
	if err != nil {
		return nil, err
	}

	genres := GenericResource{
		// TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       item.GetName(),
			GenerateName:               item.GetGenerateName(),
			Namespace:                  item.GetNamespace(),
			SelfLink:                   "",
			UID:                        item.GetUID(),
			ResourceVersion:            item.GetResourceVersion(),
			Generation:                 item.GetGeneration(),
			CreationTimestamp:          item.GetCreationTimestamp(),
			DeletionTimestamp:          item.GetDeletionTimestamp(),
			DeletionGracePeriodSeconds: item.GetDeletionGracePeriodSeconds(),
			Labels:                     item.GetLabels(),
			Annotations:                item.GetAnnotations(),
			OwnerReferences:            item.GetOwnerReferences(),
			Finalizers:                 item.GetFinalizers(),
			ClusterName:                item.GetClusterName(),
			// ManagedFields:              nil,
		},
		Spec: GenericResourceSpec{
			Group:                gvk.Group,
			Version:              gvk.Version,
			Kind:                 gvk.Kind,
			Replicas:             0,
			RoleReplicas:         nil,
			Mode:                 "",
			TotalResource:        core.ResourceRequirements{},
			AppResource:          core.ResourceRequirements{},
			RoleResourceLimits:   nil,
			RoleResourceRequests: nil,
			// Status:               "",
		},
		Status: *itemStatus,
	}

	{
		rv, err := resourcemetrics.Replicas(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.Replicas = rv
	}
	{
		rv, err := resourcemetrics.RoleReplicas(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.RoleReplicas = rv
	}
	{
		rv, err := resourcemetrics.Mode(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.Mode = rv
	}
	{
		rv, err := resourcemetrics.TotalResourceRequests(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.TotalResource.Requests = rv
	}
	{
		rv, err := resourcemetrics.TotalResourceLimits(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.TotalResource.Limits = rv
	}
	{
		rv, err := resourcemetrics.AppResourceRequests(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.AppResource.Requests = rv
	}
	{
		rv, err := resourcemetrics.AppResourceLimits(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.AppResource.Limits = rv
	}
	{
		rv, err := resourcemetrics.RoleResourceRequests(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.RoleResourceRequests = rv
	}
	{
		rv, err := resourcemetrics.RoleResourceLimits(content)
		if err != nil {
			return nil, err
		}
		genres.Spec.RoleResourceLimits = rv
	}
	return &genres, nil
}
