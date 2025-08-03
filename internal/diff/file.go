package diff

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

func expandFilenames(filenames []string, recursive bool) ([]string, error) {
	var res []string

	for _, filename := range filenames {
		stat, err := os.Stat(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", filename, err)
		}
		if !stat.IsDir() {
			res = append(res, filename)
			continue
		}
		// dir
		if recursive {
			err := filepath.WalkDir(filename, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
					res = append(res, path)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}

	return res, nil
}

func processFile(filename string, opts *Options, dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) (bool, error) {
	f, err := os.Open(filename)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s: %w", filename, err)
	}

	hasDiff := false
	for obj := range objectsFromYaml(f) {
		if obj == nil {
			return false, errors.New("failed to decode YAML")
		}

		diffFound, err := diffObject(obj, opts, dynamicClient, discoveryClient)
		if err != nil {
			return false, fmt.Errorf("failed to diff object %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
		if diffFound {
			hasDiff = true
		}
	}

	return hasDiff, nil
}

func objectsFromYaml(r io.Reader) chan *unstructured.Unstructured {
	ch := make(chan *unstructured.Unstructured)
	go func() {
		defer close(ch)
		decoder := yaml.NewYAMLOrJSONDecoder(r, 4096)
		for {
			var obj unstructured.Unstructured
			err := decoder.Decode(&obj)
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				fmt.Fprintf(os.Stderr, "error decoding YAML: %v\n", err)
				ch <- nil
				return
			}

			if obj.GetKind() == "" {
				continue
			}

			ch <- &obj
		}
	}()
	return ch
}
