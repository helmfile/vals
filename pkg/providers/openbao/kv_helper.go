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
// If mountPath is provided, it is used as the mount boundary; otherwise, the first path
// segment is treated as the mount path. Leading and trailing slashes are trimmed.
func resolveKVDataPath(key, mountPath string) (dataPath string, v1Path string) {
	cleanKey := strings.Trim(key, "/")
	cleanMount := strings.Trim(mountPath, "/")

	var mount, subpath string
	if cleanMount != "" {
		mount = cleanMount
		prefix := cleanMount + "/"
		if strings.HasPrefix(cleanKey, prefix) {
			subpath = strings.TrimPrefix(cleanKey, prefix)
		} else if cleanKey == cleanMount {
			subpath = ""
		} else {
			subpath = cleanKey
		}
	} else {
		parts := strings.Split(cleanKey, "/")
		mount = parts[0]
		if len(parts) > 1 {
			subpath = strings.Join(parts[1:], "/")
		}
	}

	// KV v1 path
	if subpath == "" {
		v1Path = mount
	} else {
		v1Path = path.Join(mount, subpath)
	}

	// KV v2 path: standard data path is <mount>/data/<subpath>
	switch {
	case subpath == "":
		dataPath = path.Join(mount, "data")
	case subpath == "data" || strings.HasPrefix(subpath, "data/"):
		dataPath = path.Join(mount, subpath)
	default:
		dataPath = path.Join(mount, "data", subpath)
	}

	return dataPath, v1Path
}
