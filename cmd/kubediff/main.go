package main

import (
	"fmt"
	"github.com/sepich/kubediff/internal/filter"
	"github.com/sepich/kubediff/internal/store"
	"os"

	"github.com/prometheus/common/version"
	"github.com/sepich/kubediff/internal/diff"
	"github.com/spf13/pflag"
)

func main() {
	var err error
	d := &diff.Diff{}
	var filename = pflag.StringSliceP("filename", "f", []string{}, "Filename or directory with files to compare")
	var recursive = pflag.BoolP("recursive", "R", false, "Process the directory used in -f, --filename recursively")
	pflag.BoolVarP(&d.SkipSecrets, "skip-secrets", "", false, "Skip comparing of Secrets (no permission to read them)")
	pflag.StringVar(&d.Cluster, "cluster", "", "The name of the kubeconfig cluster to use")
	pflag.StringVar(&d.Context, "context", "", "The name of the kubeconfig context to use")
	pflag.StringVar(&d.Kubeconfig, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests")
	pflag.StringVarP(&d.Namespace, "namespace", "n", "", "If present, the namespace scope for this CLI request")
	pflag.StringVar(&d.Token, "token", "", "Bearer token for authentication to the API server")
	var filterfile = pflag.StringP("filter-file", "", "", "Path to a filter yml file to apply defaults before comparing (default built-in)")
	var ver = pflag.BoolP("version", "v", false, "Show version and exit")
	pflag.Parse()
	if *ver {
		fmt.Println(version.Print("kubediff"))
		os.Exit(0)
	}

	if len(*filename) == 0 {
		fmt.Fprintf(os.Stderr, "Error: must specify at least one filename\n")
		os.Exit(2)
	}
	if d.Files, err = store.ExpandToFilenames(*filename, *recursive); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read dir: %v\n", err)
		os.Exit(2)
	}

	d.Filter, err = filter.NewFilter(*filterfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read filter-file: %v\n", err)
		os.Exit(2)
	}

	exitCode, err := d.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	os.Exit(exitCode)
}
