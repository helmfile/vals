package openbao

import (
	"path"
	"strings"
)

// resolveKVDataPath resolves a secret key into its standard KV v2 secret data path
// ("/v1/<mount>/data/<path>") and KV v1 secret path ("/v1/<mount>/<path>").
//
// For KV v2, the standard secret engine data path is "<mount>/data/<path>".
// If the key already contains the "/data/" segment (e.g. "<mount>/data/<path>"),
// it is returned as-is to avoid duplicate data prefixes.
//
// Leading and trailing slashes are trimmed.
func resolveKVDataPath(key, mountPath string) (dataPath string, v1Path string) {
	mount, subpath := splitMountPath(strings.Trim(key, "/"), strings.Trim(mountPath, "/"))

	v1Path = path.Join(mount, subpath)

	dataPath = path.Join(mount, "data", subpath)
	if subpath == "data" || strings.HasPrefix(subpath, "data/") {
		dataPath = path.Join(mount, subpath)
	}

	return dataPath, v1Path
}

// splitMountPath splits key into its mount and subpath components.
//
// When mountPath is empty, the first path segment of key is the mount and the
// rest is the subpath. Otherwise mountPath is the mount and the subpath is key
// with the "mountPath/" prefix stripped — or the whole key when it does not
// carry the prefix.
func splitMountPath(key, mountPath string) (mount, subpath string) {
	if mountPath == "" {
		mount, subpath, _ = strings.Cut(key, "/")
		return mount, subpath
	}
	if key == mountPath {
		return mountPath, ""
	}
	return mountPath, strings.TrimPrefix(key, mountPath+"/")
}
