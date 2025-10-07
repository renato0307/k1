package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs for args parsing
type TestSimpleArgs struct {
	Name  string `form:"name" title:"Name" default:"test" optional:"true"`
	Count int    `form:"count" title:"Count" default:"5" optional:"true"`
}

type TestOptionalArgs struct {
	Required string `form:"req" title:"Required"`
	Optional string `form:"opt" title:"Optional" optional:"true" default:"default"`
}

type TestBoolArgs struct {
	Enabled bool `form:"enabled" title:"Enabled" default:"true" optional:"true"`
}

func TestParseInlineArgs_Simple(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TestSimpleArgs
		wantErr  bool
	}{
		{
			name:  "both args provided",
			input: "myname 10",
			expected: TestSimpleArgs{
				Name:  "myname",
				Count: 10,
			},
			wantErr: false,
		},
		{
			name:  "use defaults",
			input: "",
			expected: TestSimpleArgs{
				Name:  "test",
				Count: 5,
			},
			wantErr: false,
		},
		{
			name:  "partial args",
			input: "custom",
			expected: TestSimpleArgs{
				Name:  "custom",
				Count: 5, // default
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result TestSimpleArgs
			err := ParseInlineArgs(&result, tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Count, result.Count)
			}
		})
	}
}

func TestParseInlineArgs_Optional(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TestOptionalArgs
		wantErr  bool
	}{
		{
			name:  "both provided",
			input: "required optional",
			expected: TestOptionalArgs{
				Required: "required",
				Optional: "optional",
			},
			wantErr: false,
		},
		{
			name:  "only required",
			input: "required",
			expected: TestOptionalArgs{
				Required: "required",
				Optional: "default",
			},
			wantErr: false,
		},
		{
			name:     "missing required",
			input:    "",
			expected: TestOptionalArgs{},
			wantErr:  true, // Should error on missing required field
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result TestOptionalArgs
			err := ParseInlineArgs(&result, tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Required, result.Required)
				assert.Equal(t, tt.expected.Optional, result.Optional)
			}
		})
	}
}

func TestParseInlineArgs_Bool(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		wantErr  bool
	}{
		{
			name:     "true value",
			input:    "true",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "false value",
			input:    "false",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "default value",
			input:    "",
			expected: true, // default from struct tag
			wantErr:  false,
		},
		{
			name:     "invalid bool",
			input:    "notabool",
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result TestBoolArgs
			err := ParseInlineArgs(&result, tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result.Enabled)
			}
		})
	}
}

func TestParseInlineArgs_RealStructs(t *testing.T) {
	t.Run("ScaleArgs", func(t *testing.T) {
		var args ScaleArgs
		err := ParseInlineArgs(&args, "5")
		require.NoError(t, err)
		assert.Equal(t, 5, args.Replicas)
	})

	t.Run("LogsArgs with defaults", func(t *testing.T) {
		var args LogsArgs
		err := ParseInlineArgs(&args, "")
		require.NoError(t, err)
		assert.Equal(t, 100, args.Tail)
		assert.Equal(t, false, args.Follow)
	})

	t.Run("DrainArgs with all fields", func(t *testing.T) {
		var args DrainArgs
		err := ParseInlineArgs(&args, "60 true false")
		require.NoError(t, err)
		assert.Equal(t, 60, args.GracePeriod)
		assert.Equal(t, true, args.Force)
		assert.Equal(t, false, args.IgnoreDaemonsets)
	})
}

func TestGenerateInputFields(t *testing.T) {
	tests := []struct {
		name          string
		argsType      any
		expectFields  int
		expectNoError bool
	}{
		{
			name:          "TestSimpleArgs",
			argsType:      &TestSimpleArgs{},
			expectFields:  2, // Name and Count
			expectNoError: true,
		},
		{
			name:          "ScaleArgs",
			argsType:      &ScaleArgs{},
			expectFields:  1, // Replicas
			expectNoError: true,
		},
		{
			name:          "LogsArgs",
			argsType:      &LogsArgs{},
			expectFields:  3, // Container, Tail, Follow
			expectNoError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := GenerateInputFields(tt.argsType)

			if tt.expectNoError {
				require.NoError(t, err)
				assert.Len(t, fields, tt.expectFields)

				// Verify all fields have titles
				for _, field := range fields {
					assert.NotEmpty(t, field.Label)
				}
			} else {
				require.Error(t, err)
			}
		})
	}
}
