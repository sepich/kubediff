package diff

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExecuteDiff(t *testing.T) {
	fileObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-config",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"key1": "value1",
			},
		},
	}

	tests := []struct {
		name          string
		clusterObj    *unstructured.Unstructured
		envDiffCmd    string
		expectedDiff  bool
		expectedError bool
	}{
		{
			name: "no differences - identical objects",
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-config",
						"namespace": "default",
					},
					"data": map[string]interface{}{
						"key1": "value1",
					},
				},
			},
			envDiffCmd:    "",
			expectedDiff:  false,
			expectedError: false,
		},
		{
			name: "differences found - different data",
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-config",
						"namespace": "default",
					},
					"data": map[string]interface{}{
						"key1": "value2", // Different value
					},
				},
			},
			envDiffCmd:    "",
			expectedDiff:  true,
			expectedError: false,
		},
		{
			name: "differences found - empty cluster object",
			clusterObj: &unstructured.Unstructured{
				Object: nil, // Empty cluster object (not found)
			},
			envDiffCmd:    "",
			expectedDiff:  true,
			expectedError: false,
		},
		{
			name: "differences found - another command",
			clusterObj: &unstructured.Unstructured{
				Object: nil, // Empty cluster object (not found)
			},
			envDiffCmd:    "diff -y",
			expectedDiff:  true,
			expectedError: false,
		},
		{
			name: "error - invalid diff command",
			clusterObj: &unstructured.Unstructured{
				Object: nil,
			},
			envDiffCmd:    "nonexistent-diff-command",
			expectedDiff:  false,
			expectedError: true,
		},
		{
			name: "error - invalid cluster object marshal",
			clusterObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": func() {}, // Invalid type that can't be marshaled
					},
				},
			},
			envDiffCmd:    "",
			expectedDiff:  false,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envDiffCmd != "" {
				os.Setenv("KUBECTL_EXTERNAL_DIFF", tt.envDiffCmd)
			}

			originalStdout := os.Stdout
			_, w, _ := os.Pipe()
			os.Stdout = w
			defer func() {
				w.Close()
				os.Stdout = originalStdout
			}()

			gotDiff, gotErr := executeDiff(fileObj, tt.clusterObj)
			if tt.expectedError && gotErr == nil {
				t.Errorf("%s: executeDiff() expected error but got nil", tt.name)
			}
			if !tt.expectedError && gotErr != nil {
				t.Errorf("%s: executeDiff() unexpected error = %v", tt.name, gotErr)
			}
			if gotDiff != tt.expectedDiff {
				t.Errorf("%s: executeDiff() diff = %v, expected %v", tt.name, gotDiff, tt.expectedDiff)
			}
		})
	}
}
