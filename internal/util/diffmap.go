package util

func ExtractResourceVersion(objectAsMap map[string]any) (string, bool) {
	metadata, ok := objectAsMap["metadata"]
	if !ok {
		return "", false
	}
	metadataMap, ok := metadata.(map[string]any)
	if !ok {
		return "", false
	}
	resourceVersion, ok := metadataMap["resourceVersion"]
	if !ok {
		return "", false
	}
	resourceVersionStr, ok := resourceVersion.(string)
	return resourceVersionStr, ok
}
