package diff

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func getGVRAndScope(gvk schema.GroupVersionKind, discoveryClient discovery.DiscoveryInterface) (*schema.GroupVersionResource, bool, error) {
	apiResourceList, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return nil, false, err
	}

	for _, resource := range apiResourceList.APIResources {
		if resource.Kind == gvk.Kind {
			return &schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: resource.Name,
			}, resource.Namespaced, nil
		}
	}

	return nil, false, fmt.Errorf("resource not found for kind %s", gvk.Kind)
}
