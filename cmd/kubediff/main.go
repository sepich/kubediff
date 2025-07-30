package main

import (
	"fmt"
	"os"

	"github.com/prometheus/common/version"
	"github.com/sepich/kubediff/internal/diff"
	"github.com/spf13/pflag"
)

func main() {
	opts := &diff.Options{}

	pflag.StringSliceVarP(&opts.Filename, "filename", "f", []string{}, "Filename, directory, or URL to files to compare")
	pflag.BoolVarP(&opts.Recursive, "recursive", "R", false, "Process the directory used in -f, --filename recursively")
	pflag.StringVar(&opts.Cluster, "cluster", "", "The name of the kubeconfig cluster to use")
	pflag.StringVar(&opts.Context, "context", "", "The name of the kubeconfig context to use")
	pflag.StringVar(&opts.Kubeconfig, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests")
	pflag.StringVarP(&opts.Namespace, "namespace", "n", "", "If present, the namespace scope for this CLI request")
	pflag.StringVar(&opts.Token, "token", "", "Bearer token for authentication to the API server")
	var ver = pflag.BoolP("version", "v", false, "Show version and exit")
	pflag.Parse()
	if *ver {
		fmt.Println(version.Print("kubediff"))
		os.Exit(0)
	}

	if len(opts.Filename) == 0 {
		fmt.Fprintf(os.Stderr, "Error: must specify at least one filename\n")
		os.Exit(1)
	}

	exitCode, err := diff.Run(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}
	os.Exit(exitCode)
}
