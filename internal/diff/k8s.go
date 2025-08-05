package diff

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/util/retry"
)

func (d Diff) getGVRAndScope(gvk schema.GroupVersionKind, discoveryClient discovery.DiscoveryInterface) (*schema.GroupVersionResource, bool, error) {
	key := gvk.GroupVersion().String()
	res, ok := d.apiResourceList[key]

	// cache response for all further diff objects
	if !ok {
		var err error
		err = retry.OnError(retry.DefaultRetry, func(err error) bool {
			if isRetriableError(err) {
				fmt.Fprintf(os.Stderr, "Get resources for %s error: %v, will retry...\n", key, err)
				return true
			}
			return false
		}, func() error {
			res, err = discoveryClient.ServerResourcesForGroupVersion(key)
			return err
		})
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
