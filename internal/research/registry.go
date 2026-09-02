package research

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxManifestBytes = 4 << 20

var ErrDatasetNotFound = errors.New("research dataset is not registered")

// Registry is an immutable in-memory index of registered frozen datasets whose
// manifests passed structural and integrity checks; registration is not
// evidence of scientific validation.
type Registry struct {
	manifests map[string]DatasetManifest
	ids       []string
}

func NewRegistry(manifests []DatasetManifest) (*Registry, error) {
	registry := &Registry{manifests: make(map[string]DatasetManifest, len(manifests))}
	for i, manifest := range manifests {
		if err := ValidateDatasetManifest(manifest); err != nil {
			return nil, fmt.Errorf("manifest %d: %w", i, err)
		}
		if _, duplicate := registry.manifests[manifest.DatasetID]; duplicate {
			return nil, fmt.Errorf("duplicate dataset_id %q", manifest.DatasetID)
		}
		registry.manifests[manifest.DatasetID] = cloneManifest(manifest)
		registry.ids = append(registry.ids, manifest.DatasetID)
	}
	if len(registry.manifests) == 0 {
		return nil, errors.New("registry requires at least one registered dataset manifest")
	}
	sort.Strings(registry.ids)
	return registry, nil
}

// LoadRegistry reads only regular *.json files immediately inside dir. Symlinks
// and nested paths are rejected to keep provenance loading inside the configured
// manifest root.
func LoadRegistry(dir string) (*Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read manifest directory: %w", err)
	}
	var manifests []DatasetManifest
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil, fmt.Errorf("manifest %q must be a regular file", entry.Name())
		}
		manifestPath := filepath.Join(dir, entry.Name())
		data, err := readLimitedFile(manifestPath, maxManifestBytes)
		if err != nil {
			return nil, fmt.Errorf("read manifest %q: %w", entry.Name(), err)
		}
		var manifest DatasetManifest
		if err := decodeStrictJSON(data, &manifest); err != nil {
			return nil, fmt.Errorf("decode manifest %q: %w", entry.Name(), err)
		}
		manifests = append(manifests, manifest)
	}
	return NewRegistry(manifests)
}

// LoadDatasetManifest loads one strict manifest without scanning sibling JSON
// files. It is used by the explicit SQLite materialization command, where the
// source artifact directory also contains non-registry metadata.
func LoadDatasetManifest(path string) (DatasetManifest, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return DatasetManifest{}, fmt.Errorf("inspect manifest: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return DatasetManifest{}, errors.New("manifest must be a regular non-symlink file")
	}
	data, err := readLimitedFile(path, maxManifestBytes)
	if err != nil {
		return DatasetManifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest DatasetManifest
	if err := decodeStrictJSON(data, &manifest); err != nil {
		return DatasetManifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := ValidateDatasetManifest(manifest); err != nil {
		return DatasetManifest{}, fmt.Errorf("validate manifest: %w", err)
	}
	return manifest, nil
}

func (r *Registry) List() []DatasetManifest {
	if r == nil {
		return nil
	}
	result := make([]DatasetManifest, 0, len(r.ids))
	for _, id := range r.ids {
		result = append(result, cloneManifest(r.manifests[id]))
	}
	return result
}

// Len answers health/readiness checks without cloning every manifest. Registry
// contents are immutable after construction, so this is safe without locking.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.ids)
}

func (r *Registry) Get(datasetID string) (DatasetManifest, bool) {
	if r == nil {
		return DatasetManifest{}, false
	}
	manifest, ok := r.manifests[datasetID]
	if !ok {
		return DatasetManifest{}, false
	}
	return cloneManifest(manifest), true
}

func (r *Registry) Require(datasetID string) (DatasetManifest, error) {
	manifest, ok := r.Get(datasetID)
	if !ok {
		return DatasetManifest{}, fmt.Errorf("%w: %s", ErrDatasetNotFound, datasetID)
	}
	return manifest, nil
}

// ResolveAnalysisRequest is the dataset/protocol boundary used before queueing
// or executing an analysis. It deliberately does not derive analysis_id: that
// identifier also binds the exact SoftwareReference and therefore belongs at
// the execution boundary that owns the clean runtime commit.
func (r *Registry) ResolveAnalysisRequest(request AnalysisRequest) (DatasetManifest, AnalysisRequest, error) {
	normalized, err := NormalizeAnalysisRequest(request)
	if err != nil {
		return DatasetManifest{}, AnalysisRequest{}, err
	}
	manifest, err := r.Require(normalized.DatasetID)
	if err != nil {
		return DatasetManifest{}, AnalysisRequest{}, err
	}
	if err := validateAnalysisRequestAgainstManifest(manifest, normalized); err != nil {
		return DatasetManifest{}, AnalysisRequest{}, err
	}
	return manifest, normalized, nil
}

func cloneManifest(manifest DatasetManifest) DatasetManifest {
	data, err := json.Marshal(manifest)
	if err != nil {
		panic("research manifest contract contains an unsupported JSON value: " + err.Error())
	}
	var cloned DatasetManifest
	if err := json.Unmarshal(data, &cloned); err != nil {
		panic("research manifest clone failed: " + err.Error())
	}
	return cloned
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func readLimitedFile(path string, limit int64) ([]byte, error) {
	// #nosec G304 -- callers scope this path to a configured registry/store root and reject symlinks before opening it.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, nil
}
