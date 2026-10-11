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
// If mountPath is provided, it is used as the mount boundary and the remainder of
// the key becomes the subpath. Otherwise, the first path segment is treated as
// the mount. Leading and trailing slashes are trimmed.
func resolveKVDataPath(key, mountPath string) (dataPath string, v1Path string) {
	cleanKey := strings.Trim(key, "/")
	cleanMount := strings.Trim(mountPath, "/")

	var mount, subpath string
	if cleanMount == "" {
		mount, subpath, _ = strings.Cut(cleanKey, "/")
	} else if trimmed := strings.TrimPrefix(cleanKey, cleanMount+"/"); trimmed != cleanKey {
		mount, subpath = cleanMount, trimmed
	} else if cleanMount == cleanKey {
		mount, subpath = cleanMount, ""
	} else {
		mount, subpath = cleanMount, cleanKey
	}

	v1Path = path.Join(mount, subpath)

	switch {
	case subpath == "data" || strings.HasPrefix(subpath, "data/"):
		dataPath = path.Join(mount, subpath)
	default:
		dataPath = path.Join(mount, "data", subpath)
	}

	return dataPath, v1Path
}
