package main

import (
	"context"
	"fmt"

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

	var nodes core.NodeList
	err = c.List(context.TODO(), &nodes)
	if err != nil {
		panic(err)
	}
	for _, n := range nodes.Items {
		fmt.Println(n.Name)
	}

	return calculate(cfg, sets.NewString("kubedb.com"))
}
