package s3

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/google/go-cmp/cmp"

	"github.com/helmfile/vals/pkg/config"
)

type mockedS3 struct {
	s3iface.S3API

	Bucket, Key string
	Output      *s3.GetObjectOutput
	Error       awserr.Error
}

func Output(b string) *s3.GetObjectOutput {
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(b)),
	}
}

func (m mockedS3) GetObject(in *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	bucket := *in.Bucket
	if bucket != m.Bucket {
		return nil, fmt.Errorf("unexpected bucket: %s", bucket)
	}

	key := *in.Key
	if key != m.Key {
		return nil, fmt.Errorf("unexpected key: %s", key)
	}

	return m.Output, m.Error
}

func TestGetString(t *testing.T) {
	cases := []struct {
		key     string
		want    string
		wantErr string

		s3 mockedS3
	}{
		{
			key:     "foo/missing",
			wantErr: "getting s3 object: NoSuchKey: no such key\ncaused by: simulated no-such-key error",
			s3: mockedS3{
				Bucket: "foo",
				Key:    "missing",
				Output: nil,
				Error:  awserr.New(s3.ErrCodeNoSuchKey, "no such key", errors.New("simulated no-such-key error")),
			},
		},
		{
			key:  "foo/bar",
			want: `{"mysecret":"value"}`,
			s3: mockedS3{
				Bucket: "foo",
				Key:    "bar",
				Output: Output(`{"mysecret":"value"}`),
				Error:  nil,
			},
		},
	}

	for _, c := range cases {
		p := New(config.MapConfig{M: map[string]interface{}{}})

		p.s3Client = c.s3

		got, err := p.GetString(c.key)

		if err != nil {
			if err.Error() != c.wantErr {
				t.Fatalf("unexpected error: want %q, got %q", c.wantErr, err.Error())
			}
		} else {
			if c.wantErr != "" {
				t.Fatalf("expected error did not occur: want %q, got none", c.wantErr)
			}
		}

		if got != c.want {
			t.Errorf("unexpected result: want %q, got %q", c.want, got)
		}
	}
}

func TestGetStringMap(t *testing.T) {
	cases := []struct {
		key     string
		want    map[string]interface{}
		wantErr string

		s3 mockedS3
	}{
		{
			key:     "foo/missing",
			wantErr: "getting s3 object: NoSuchKey: no such key\ncaused by: simulated no-such-key error",
			s3: mockedS3{
				Bucket: "foo",
				Key:    "missing",
				Output: nil,
				Error:  awserr.New(s3.ErrCodeNoSuchKey, "no such key", errors.New("simulated no-such-key error")),
			},
		},
		{
			key:  "foo/bar",
			want: map[string]interface{}{"mysecret": "value"},
			s3: mockedS3{
				Bucket: "foo",
				Key:    "bar",
				Output: Output(`{"mysecret":"value"}`),
				Error:  nil,
			},
		},
	}

	for _, c := range cases {
		p := New(config.MapConfig{M: map[string]interface{}{}})

		p.s3Client = c.s3

		got, err := p.GetStringMap(c.key)

		if err != nil {
			if err.Error() != c.wantErr {
				t.Fatalf("unexpected error: want %q, got %q", c.wantErr, err.Error())
			}
		} else {
			if c.wantErr != "" {
				t.Fatalf("expected error did not occur: want %q, got none", c.wantErr)
			}
		}

		if diff := cmp.Diff(c.want, got); diff != "" {
			t.Errorf("unexpected result: -(want), +(got)\n%s", diff)
		}
	}
}
