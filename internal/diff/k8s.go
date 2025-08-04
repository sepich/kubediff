package diff

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func (d Diff) getGVRAndScope(gvk schema.GroupVersionKind, discoveryClient discovery.DiscoveryInterface) (*schema.GroupVersionResource, bool, error) {
	key := gvk.GroupVersion().String()
	res, ok := d.apiResourceList[key]

	// cache response for all further diff objects
	if !ok {
		var err error
		res, err = discoveryClient.ServerResourcesForGroupVersion(key)
		if err != nil {
			return nil, false, err
		}
		d.apiResourceList[key] = res
	}

	for _, resource := range res.APIResources {
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
