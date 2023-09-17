package file

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/helmfile/vals/pkg/config"
)

const textFileContent = "file content"

const yamlFileContent = `
---
foo:
  bar: baz
`

// Mock implementation of os.ReadFile for testing
func mockReadFile(name string) ([]byte, error) {
	switch name {
	case "path/to/file.txt":
		return []byte(textFileContent), nil
	case "path/to/empty_file.txt":
		return []byte{}, nil
	case "path/to/file.yaml":
		return []byte(yamlFileContent), nil
	case "path/to/error_file.txt":
		return nil, errors.New("error reading file")
	default:
		return nil, errors.New("file not found")
	}
}

func Test_provider_GetString(t *testing.T) {
	type params struct {
		Encode string
	}
	type args struct {
		key string
	}
	tests := []struct {
		name    string
		params  params
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Encode parameter is empty",
			params: params{
				Encode: "",
			},
			args: args{
				key: "path/to/file.txt",
			},
			want:    textFileContent,
			wantErr: false,
		},
		{
			name: "Encode parameter is 'raw'",
			params: params{
				Encode: "raw",
			},
			args: args{
				key: "path/to/file.txt",
			},
			want:    textFileContent,
			wantErr: false,
		},
		{
			name: "Encode parameter is 'base64'",
			params: params{
				Encode: "base64",
			},
			args: args{
				key: "path/to/file.txt",
			},
			want:    base64.StdEncoding.EncodeToString([]byte(textFileContent)),
			wantErr: false,
		},
		{
			name: "File is empty",
			params: params{
				Encode: "raw",
			},
			args: args{
				key: "path/to/empty_file.txt",
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "Error reading file",
			params: params{
				Encode: "raw",
			},
			args: args{
				key: "path/to/error_file.txt",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "File not found",
			params: params{
				Encode: "raw",
			},
			args: args{
				key: "path/to/nonexistent_file.txt",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create provider with mock
			conf := map[string]interface{}{}
			conf["encode"] = tt.params.Encode
			p := New(config.MapConfig{M: conf})
			p.fileReader = mockReadFile

			got, err := p.GetString(tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("provider.GetString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("provider.GetString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_provider_GetStringMap(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "Unmarshal valid yaml file",
			args: args{
				key: "path/to/file.yaml",
			},
			want: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			},
			wantErr: false,
		},
		{
			name: "Unmarshal invalid yaml file",
			args: args{
				key: "path/to/file.txt",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "File is empty",
			args: args{
				key: "path/to/empty_file.txt",
			},
			want:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "Error reading file",
			args: args{
				key: "path/to/error_file.txt",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "File not found",
			args: args{
				key: "path/to/nonexistent_file.txt",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := map[string]interface{}{}
			p := New(config.MapConfig{M: conf})
			p.fileReader = mockReadFile

			got, err := p.GetStringMap(tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("provider.GetStringMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("provider.GetStringMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
