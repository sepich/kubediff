package diff

import (
	"errors"
	"fmt"
	"github.com/sepich/kubediff/internal/store"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"os"
)

func (d *Diff) processFile(filename string, dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	f, err := os.Open(filename)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s: %w", filename, err)
	}

	hasDiff := false
	for obj := range store.YamlToObj(f) {
		if obj == nil {
			return false, errors.New("failed to decode YAML")
		}

		diffFound, err := d.diffObject(obj, dynamicClient, discoveryClient)
		if err != nil {
			return false, fmt.Errorf("failed to diff object %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
		if diffFound {
			hasDiff = true
		}
	}

	return hasDiff, nil
}
