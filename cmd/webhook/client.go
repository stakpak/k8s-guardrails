package main

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	client          dynamic.Interface
	discoveryClient *discovery.DiscoveryClient
)

func initClient() {
	config, err := ctrl.GetConfig()
	if err != nil {
		panic(err)
	}
	client, err = dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	discoveryClient, err = discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		panic(err)
	}
}

func GetObjectLabels(apiVersion string, kind string, namespace string, name string) (map[string]string, error) {
	ctx := context.Background()
	var err error

	klog.V(1).Info(apiVersion, kind, namespace, name)
	klog.V(1).Info(fmt.Sprintf("Owner %s %s %s %s", apiVersion, kind, namespace, name))

	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("expected apiVersion to be made of 2 parts but got: %s", apiVersion)
	}

	apiGroupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("failed while getting discovery api group resources")
	}

	restMapper := restmapper.NewDiscoveryRESTMapper(apiGroupResources)
	groupKind := schema.GroupKind{
		Group: groupVersion.Group,
		Kind:  kind,
	}

	mapping, err := restMapper.RESTMapping(groupKind, groupVersion.Version)
	if err != nil {
		return nil, fmt.Errorf("unable to find mapping for: %s %s", apiVersion, kind)
	}

	objClient := client.Resource(mapping.Resource)

	var obj *unstructured.Unstructured

	if mapping.Scope.Name() == meta.RESTScopeNamespace.Name() && namespace != "" {
		obj, err = objClient.Namespace(namespace).Get(ctx, name, v1.GetOptions{})
	} else {
		obj, err = objClient.Get(ctx, name, v1.GetOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get owner object: %s", err)
	}

	return obj.GetLabels(), nil
}
