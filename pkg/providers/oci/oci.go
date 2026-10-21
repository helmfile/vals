package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	// Keeping track of repository
	ctx        context.Context
	creds      *credentials.DynamicStore
	repository *remote.Repository
	log        *log.Logger

	// Annotation to match for layer, org.opencontainers.image.title by default
	Annotation string
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}
	p.Annotation = cfg.String("annotation")
	if p.Annotation == "" {
		p.Annotation = "org.opencontainers.image.title"
	}
	creds, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		p.log.Debugf("Failed to load Docker credentials: %v\n", err)
	}
	p.creds = creds
	return p
}

// expected format repository:TAG:org.opencontainers.image.title
func (p *provider) GetString(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	p.ctx = ctx

	// Parse the key using the global function
	registryRepo, tagDigest, filename, err := parseKey(key)
	if err != nil {
		return "", err
	}

	_, err = p.configureRepo(registryRepo)
	if err != nil {
		return "", fmt.Errorf("unable to configure repository: %w", err)
	}

	manifest, err := p.getManifest(tagDigest)
	if err != nil {
		return "", fmt.Errorf("unable to get manifest from repo: %w", err)
	}

	str, err := p.getLayerByAnnotationFromManifest(manifest, filename)
	if err != nil {
		return "", err
	}

	return str, nil
}

func (p *provider) GetStringMap(key string) (map[string]any, error) {
	yamlData, err := p.GetString(key)
	if err != nil {
		return nil, err
	}

	m := map[string]any{}

	if err := yaml.Unmarshal([]byte(yamlData), &m); err != nil {
		return nil, err
	}

	return m, nil
}

func (p *provider) configureRepo(repoString string) (registry.Repository, error) {
	repository, err := remote.NewRepository(repoString)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo: %w", err)
	}
	repository.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: func(ctx context.Context, registry string) (auth.Credential, error) {
			return p.creds.Get(ctx, registry)
		},
	}
	p.repository = repository
	return repository, nil
}

func (p *provider) getManifest(tagDigest string) (m *v1.Manifest, err error) {
	desc, reader, err := p.repository.FetchReference(p.ctx, tagDigest)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch reference from repository: %v", err)
	}
	defer func() {
		if cerr := reader.Close(); err == nil {
			err = cerr
		}
	}()
	p.log.Debugf("Found desc: %+v and read: %v", desc, reader)

	var manifestContent []byte
	manifestContent, err = io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("err reading content: %v", err)
	}

	p.log.Debugf("Parsed manifest content: %s", manifestContent)
	var manifest v1.Manifest
	if err = json.Unmarshal(manifestContent, &manifest); err != nil {
		panic(err)
	}
	return &manifest, nil
}

func (p *provider) getLayerByAnnotationFromManifest(manifest *v1.Manifest, filename string) (string, error) {
	for _, layer := range manifest.Layers {
		if layer.Annotations[p.Annotation] == filename {
			layerContent, err := content.FetchAll(p.ctx, p.repository, layer)
			if err != nil {
				return "", fmt.Errorf("unable to fetch layer from repo: %w", err)
			}
			return string(layerContent), nil
		}
	}
	return "", fmt.Errorf("unable to find layer with matching annotation: %s", p.Annotation)
}

// ParseKey parses an OCI key in the format "registry/repository:tag:filename"
// It tries different colon positions from right to left until a valid reference is found
func parseKey(key string) (registryRepo, tagDigest, filename string, err error) {
	// Start from the rightmost colon and work backwards
	remaining := key
	var filenameParts []string
	for {
		// Find the last colon in the remaining string
		lastColon := strings.LastIndex(remaining, ":")
		if lastColon == -1 {
			return "", "", "", fmt.Errorf("invalid key format, expected registry/repository:tag:filename got: %s", key)
		}

		// Split at this colon
		imageRef := remaining[:lastColon]
		filenameParts = append([]string{remaining[lastColon+1:]}, filenameParts...)

		// Try to parse this as a valid image reference
		ref, err := registry.ParseReference(imageRef)
		if err == nil {
			registryRepo = ref.Registry + "/" + ref.Repository
			return registryRepo, ref.Reference, strings.Join(filenameParts, ":"), nil
		}

		remaining = imageRef

		// If there are no more colons to try, return error
		if !strings.Contains(remaining, ":") {
			return "", "", "", fmt.Errorf("invalid key format, expected registry/repository:tag:filename got: %s", key)
		}
	}
}
