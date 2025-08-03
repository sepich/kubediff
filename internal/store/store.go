package store

import (
	"fmt"
	"io"
	"io/fs"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"path/filepath"
	"strings"
)

func YamlToObj(r io.Reader) chan *unstructured.Unstructured {
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

func ExpandToFilenames(names []string, recursive bool) ([]string, error) {
	var res []string

	for _, filename := range names {
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
