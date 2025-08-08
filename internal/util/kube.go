package util

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func ParseGroupVersionResource(gv string) (schema.GroupVersionResource, error) {
	parts := strings.Split(gv, "/")
	if len(parts) == 2 {
		// assume it's a resource without a group (e.g. "v1/pods")
		return schema.GroupVersionResource{
			Version:  parts[0],
			Resource: parts[1],
		}, nil
	}
	if len(parts) == 3 {
		// assume it's a resource with a group (e.g. "apps/v1/deployments")
		return schema.GroupVersionResource{
			Group:    parts[0],
			Version:  parts[1],
			Resource: parts[2],
		}, nil
	}
	// no idea...
	return schema.GroupVersionResource{}, fmt.Errorf("invalid group/version/resource format: %s", gv)
}
