package core

import (
	"encoding/json"
	"fmt"

	koanfJSON "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"gorm.io/datatypes"
)

// CalculateScaledSize calculates the scaled storage size based on redundancy metadata.
// Actual redundancy is calculated as totalSectors/minShards.
// The scaled size is dataSize * (actualRedundancy / DEFAULT_REDUNDANCY).
// This allows quota to fairly account for storage based on actual redundancy.
func CalculateScaledSize(dataSize uint64, minShards, totalSectors uint64) uint64 {
	if minShards == 0 || totalSectors == 0 {
		return dataSize
	}
	actualRedundancy := float64(totalSectors) / float64(minShards)
	scaled := float64(dataSize) * (actualRedundancy / float64(internal.DEFAULT_REDUNDANCY))
	return uint64(scaled)
}

// SlabMetadata represents siad/renterd-style storage redundancy metadata.
// This data is typically stored in upload.Metadata['redundancy'] as JSON.
type SlabMetadata struct {
	MinShards    uint64 `json:"minShards"`    // Minimum shards needed for recovery
	TotalSectors uint64 `json:"totalSectors"` // Total sectors stored (includes redundancy)
	DataSize     uint64 `json:"dataSize"`     // Original data size before redundancy
	TotalSize    uint64 `json:"totalSize"`    // Total size on disk with all sectors
}

// ParseRedundancyMetadata parses redundancy data from upload.Metadata['redundancy'].
// The input is expected to be a map with 'redundancy' key containing SlabMetadata fields.
func ParseRedundancyMetadata(metadata map[string]interface{}) (*SlabMetadata, error) {
	if metadata == nil {
		return nil, fmt.Errorf("nil metadata")
	}

	redundancy, ok := metadata["redundancy"]
	if !ok {
		return nil, fmt.Errorf("no redundancy field in metadata")
	}

	k := koanf.New(".")

	switch data := redundancy.(type) {
	case map[string]interface{}:
		if err := k.Load(confmap.Provider(data, ""), nil); err != nil {
			return nil, fmt.Errorf("failed to load redundancy metadata: %w", err)
		}
	case string:
		if err := k.Load(rawbytes.Provider([]byte(data)), koanfJSON.Parser()); err != nil {
			return nil, fmt.Errorf("failed to parse redundancy metadata string: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported redundancy metadata type: %T", redundancy)
	}

	var meta SlabMetadata
	meta.MinShards = uint64(k.Int("minShards"))
	meta.TotalSectors = uint64(k.Int("totalSectors"))
	meta.DataSize = uint64(k.Int("dataSize"))
	meta.TotalSize = uint64(k.Int("totalSize"))

	return &meta, nil
}

// EncodeSlabMetadata encodes a SlabMetadata into an upload's Metadata field.
// It preserves existing metadata keys and sets/updates the "redundancy" key.
func EncodeSlabMetadata(meta datatypes.JSON, slab SlabMetadata) (datatypes.JSON, error) {
	var metadataMap map[string]interface{}
	if len(meta) > 0 {
		if err := json.Unmarshal(meta, &metadataMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal existing metadata: %w", err)
		}
	}
	if metadataMap == nil {
		metadataMap = make(map[string]interface{})
	}

	metadataMap["redundancy"] = slab

	updated, err := json.Marshal(metadataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata with redundancy: %w", err)
	}

	return updated, nil
}
