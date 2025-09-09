package config

import "strings"

// NormalizeVersionTag converts any '+' to '-' in version strings to be compatible
// with container registries that do not support '+' in tags.
func NormalizeVersionTag(version string) string {
	return strings.ReplaceAll(strings.TrimSpace(version), "+", "-")
}

// NormalizeImageTag normalizes the tag portion of an image reference by
// converting '+' to '-' in the tag. If no tag is present, the image is returned
// unchanged.
func NormalizeImageTag(image string) string {
	image = strings.TrimSpace(image)
	idx := strings.LastIndex(image, ":")
	if idx == -1 {
		return image
	}
	repo := image[:idx]
	tag := image[idx+1:]
	return repo + ":" + NormalizeVersionTag(tag)
}
