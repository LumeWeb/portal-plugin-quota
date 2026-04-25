package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"gorm.io/datatypes"
)

func TestCalculateScaledSize(t *testing.T) {
	tests := []struct {
		name         string
		dataSize     uint64
		minShards    uint64
		totalSectors uint64
		want         uint64
	}{
		{
			name:         "default_redundancy_no_scaling",
			dataSize:     1000,
			minShards:    10,
			totalSectors: 30,
			want:         1000, // 30/10=3.0, 3.0/3.0=1.0, 1000*1.0=1000
		},
		{
			name:         "higher_redundancy_scales_up",
			dataSize:     1000,
			minShards:    10,
			totalSectors: 40,
			want:         1333, // 40/10=4.0, 4.0/3.0≈1.333, 1000*1.333≈1333
		},
		{
			name:         "lower_redundancy_scales_down",
			dataSize:     1000,
			minShards:    10,
			totalSectors: 20,
			want:         666, // 20/10=2.0, 2.0/3.0≈0.666, 1000*0.666≈666
		},
		{
			name:         "zero_minShards_fallback",
			dataSize:     1000,
			minShards:    0,
			totalSectors: 30,
			want:         1000,
		},
		{
			name:         "zero_totalSectors_fallback",
			dataSize:     1000,
			minShards:    10,
			totalSectors: 0,
			want:         1000,
		},
		{
			name:         "zero_dataSize",
			dataSize:     0,
			minShards:    10,
			totalSectors: 30,
			want:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateScaledSize(tt.dataSize, tt.minShards, tt.totalSectors)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateScaledSize_DefaultRedundancyValue(t *testing.T) {
	assert.Equal(t, 3.0, internal.DEFAULT_REDUNDANCY)
}

func TestParseRedundancyMetadata(t *testing.T) {
	t.Run("valid_map_input", func(t *testing.T) {
		metadata := map[string]interface{}{
			"redundancy": map[string]interface{}{
				"minShards":    10,
				"totalSectors": 30,
				"dataSize":     1000,
				"totalSize":    3000,
			},
		}

		got, err := ParseRedundancyMetadata(metadata)
		require.NoError(t, err)
		assert.Equal(t, uint64(10), got.MinShards)
		assert.Equal(t, uint64(30), got.TotalSectors)
		assert.Equal(t, uint64(1000), got.DataSize)
		assert.Equal(t, uint64(3000), got.TotalSize)
	})

	t.Run("valid_string_input", func(t *testing.T) {
		metadata := map[string]interface{}{
			"redundancy": `{"minShards":5,"totalSectors":15,"dataSize":500,"totalSize":1500}`,
		}

		got, err := ParseRedundancyMetadata(metadata)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), got.MinShards)
		assert.Equal(t, uint64(15), got.TotalSectors)
		assert.Equal(t, uint64(500), got.DataSize)
		assert.Equal(t, uint64(1500), got.TotalSize)
	})

	t.Run("nil_metadata", func(t *testing.T) {
		_, err := ParseRedundancyMetadata(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil metadata")
	})

	t.Run("missing_redundancy_key", func(t *testing.T) {
		metadata := map[string]interface{}{
			"other": "value",
		}
		_, err := ParseRedundancyMetadata(metadata)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no redundancy field")
	})

	t.Run("unsupported_type", func(t *testing.T) {
		metadata := map[string]interface{}{
			"redundancy": 42,
		}
		_, err := ParseRedundancyMetadata(metadata)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported redundancy metadata type")
	})

	t.Run("invalid_json_string", func(t *testing.T) {
		metadata := map[string]interface{}{
			"redundancy": "{invalid json}",
		}
		_, err := ParseRedundancyMetadata(metadata)
		assert.Error(t, err)
	})
}

func TestEncodeSlabMetadata(t *testing.T) {
	t.Run("encode_into_empty_metadata", func(t *testing.T) {
		meta := datatypes.JSON(nil)
		slab := SlabMetadata{
			MinShards:    10,
			TotalSectors: 30,
			DataSize:     1000,
			TotalSize:    3000,
		}

		result, err := EncodeSlabMetadata(meta, slab)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(result, &parsed))

		redundancy, ok := parsed["redundancy"]
		require.True(t, ok)

		got, err := ParseRedundancyMetadata(parsed)
		require.NoError(t, err)
		assert.Equal(t, slab, *got)
		_ = redundancy
	})

	t.Run("encode_preserves_existing_keys", func(t *testing.T) {
		existing := datatypes.JSON(`{"foo":"bar","baz":123}`)
		slab := SlabMetadata{
			MinShards:    5,
			TotalSectors: 15,
			DataSize:     500,
			TotalSize:    1500,
		}

		result, err := EncodeSlabMetadata(existing, slab)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(result, &parsed))

		assert.Equal(t, "bar", parsed["foo"])
		assert.True(t, parsed["redundancy"] != nil)
	})

	t.Run("encode_overwrites_existing_redundancy", func(t *testing.T) {
		existing := datatypes.JSON(`{"redundancy":{"minShards":1,"totalSectors":1,"dataSize":1,"totalSize":1}}`)
		slab := SlabMetadata{
			MinShards:    10,
			TotalSectors: 30,
			DataSize:     1000,
			TotalSize:    3000,
		}

		result, err := EncodeSlabMetadata(existing, slab)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(result, &parsed))

		got, err := ParseRedundancyMetadata(parsed)
		require.NoError(t, err)
		assert.Equal(t, slab, *got)
	})

	t.Run("invalid_existing_metadata", func(t *testing.T) {
		existing := datatypes.JSON(`not json`)
		slab := SlabMetadata{MinShards: 1}

		_, err := EncodeSlabMetadata(existing, slab)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal existing metadata")
	})
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := SlabMetadata{
		MinShards:    10,
		TotalSectors: 30,
		DataSize:     1000,
		TotalSize:    3000,
	}

	encoded, err := EncodeSlabMetadata(nil, original)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(encoded, &parsed))

	decoded, err := ParseRedundancyMetadata(parsed)
	require.NoError(t, err)
	assert.Equal(t, original, *decoded)
}
