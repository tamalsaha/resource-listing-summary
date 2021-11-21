package main

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	kubedbscheme "kubedb.dev/apimachinery/client/clientset/versioned/scheme"
	schemav1alpha1 "kubedb.dev/schema-manager/apis/schema/v1alpha1"
	kubevaultscheme "kubevault.dev/apimachinery/client/clientset/versioned/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	/*
	   k8s.io/group: apiextensions.k8s.io
	   k8s.io/kind: CustomResourceDefinition
	   k8s.io/resource: customresourcedefinitions
	   k8s.io/version: v1

	*/
	ls := metav1.LabelSelector{
		//MatchLabels: map[string]string{
		//	"k8s.io/group": "apps",
		//},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "k8s.io/group",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"apps", "kubedb.com"},
			},
		},
	}
	s, err := metav1.LabelSelectorAsSelector(&ls)
	if err != nil {
		panic(err)
	}
	g, found := s.RequiresExactMatch("k8s.io/group")
	fmt.Println("group = ", g)
	fmt.Println("found = ", found)

	requirements, selectable := s.Requirements()
	if selectable {
		fmt.Println("selectable")
	}
	for _, r := range requirements {
		fmt.Println("key = ", r.Key())
		fmt.Println("op = ", r.Operator())
		fmt.Println("values = ", r.Values().List())
	}

	fmt.Println(GetAPIGroups(s).List())

	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = kubedbscheme.AddToScheme(scheme)
	_ = kubevaultscheme.AddToScheme(scheme)
	_ = schemav1alpha1.AddToScheme(scheme)

	ctrl.SetLogger(klogr.New())
	cfg := ctrl.GetConfigOrDie()

	mapper, err := apiutil.NewDynamicRESTMapper(cfg)
	if err != nil {
		return err
	}

	c, err := client.New(cfg, client.Options{
		Scheme: scheme,
		Mapper: mapper,
		Opts: client.WarningHandlerOptions{
			SuppressWarnings:   false,
			AllowDuplicateLogs: false,
		},
	})
	if err != nil {
		return err
	}

	var n2 unstructured.UnstructuredList
	n2.SetKind("Node")
	n2.SetAPIVersion("v1")
	err = c.List(context.TODO(), &n2)
	if err != nil {
		panic(err)
	}
	for _, n := range n2.Items {
		fmt.Println(n.GetName())
	}

	var nodes core.NodeList
	err = c.List(context.TODO(), &nodes)
	if err != nil {
		panic(err)
	}
	for _, n := range nodes.Items {
		fmt.Println(n.Name)
	}

	ki, err := GetKubernetesInfo(cfg, kubernetes.NewForConfigOrDie(cfg))
	if err != nil {
		return err
	}

	return calculate(c, ki, sets.NewString("kubedb.com"))
}
