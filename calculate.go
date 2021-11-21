package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	cu "kmodules.xyz/client-go/client"
	resourcemetrics "kmodules.xyz/resource-metrics"
	"kmodules.xyz/resource-metrics/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Stats struct {
	Count     int
	Resources core.ResourceList
}

func calculate(c client.Client, apiGroups sets.String) error {
	clusterID, err := cu.ClusterUID(c)
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

		_, err := c.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if meta.IsNoMatchError(err) {
			rsmap[gvk] = Stats{} // keep track
			continue
		} else if err != nil {
			return err
		}

		var result unstructured.UnstructuredList
		result.SetGroupVersionKind(gvk)
		if err := c.List(context.TODO(), &result); err != nil {
			return err
		}

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
