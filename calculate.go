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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	resourcemetrics "kmodules.xyz/resource-metrics"
	"kmodules.xyz/resource-metrics/api"
	"kubedb.dev/apimachinery/apis/kubedb"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type Stats struct {
	Count     int
	Resources core.ResourceList
}

func calculate(cfg *rest.Config, apiGroups sets.String) error {
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	mapper, err := apiutil.NewDynamicRESTMapper(cfg)
	if err != nil {
		return err
	}

	ns, err := client.Resource(schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}).Get(context.TODO(), "kube-system", metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterID, _, err := unstructured.NestedString(ns.UnstructuredContent(), "metadata", "uid")
	if err != nil {
		return err
	}

	rsmap := map[schema.GroupVersionKind]Stats{}
	var (
		totalCount int
		rrTotal    core.ResourceList
	)
	for _, gvk := range api.RegisteredTypes() {
		if apiGroups.Len() > 0 && !apiGroups.Has(gvk.Group) {
			continue
		}

		var mapping *meta.RESTMapping
		if gvk.Group == kubedb.GroupName {
			mapping, err = mapper.RESTMapping(gvk.GroupKind())
			if meta.IsNoMatchError(err) {
				rsmap[gvk] = Stats{} // keep track
				continue
			} else if err != nil {
				return err
			}
			gvk = mapping.GroupVersionKind // v1alpha1 or v1alpha2
		} else {
			mapping, err = mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if meta.IsNoMatchError(err) {
				rsmap[gvk] = Stats{} // keep track
				continue
			} else if err != nil {
				return err
			}
		}

		var ri dynamic.ResourceInterface
		if mapping.Scope == meta.RESTScopeNamespace {
			ri = client.Resource(mapping.Resource).Namespace(core.NamespaceAll)
		} else {
			ri = client.Resource(mapping.Resource)
		}
		if result, err := ri.List(context.TODO(), metav1.ListOptions{}); err != nil {
			return err
		} else {
			var summary core.ResourceList
			for _, item := range result.Items {
				content := item.UnstructuredContent()
				rr, err := resourcemetrics.AppResourceLimits(content)
				if err != nil {
					return err
				}
				summary = api.AddResourceList(summary, rr)
			}
			rsmap[gvk] = Stats{
				Count:     len(result.Items),
				Resources: summary,
			}
			totalCount += len(result.Items)
			rrTotal = api.AddResourceList(rrTotal, summary)
		}
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
		if rr.Count == 0 {
			_, _ = fmt.Fprintf(w, "%s\t%s\t-\t-\t-\t-\t\n", gvk.GroupVersion(), gvk.Kind)
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t\n", gvk.GroupVersion(), gvk.Kind, rr.Count, rr.Resources.Cpu(), rr.Resources.Memory(), rr.Resources.Storage())
		}
	}
	_, _ = fmt.Fprintf(w, "TOTAL\t=\t%d\t%s\t%s\t%s\t\n", totalCount, rrTotal.Cpu(), rrTotal.Memory(), rrTotal.Storage())
	return w.Flush()
}
