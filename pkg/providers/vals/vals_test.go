package vals

import "testing"

const rightYaml = `
baz:
  mykey: myvalue
  password: |-
    AQICAHjdCdF7cI/3fnsYnEkQOFPNfy7fsifEtmi2Vj0qXk61IQFSxnJPmQf6cw8AvC6UPV
    rk3s3+zUORdvxB
  array:
  - pwd: |-
      AQICAHjdCdF7cI/3fnsYnEkQOFPNfy7fsifEtmi2Vj0qXk61IQFSxnJPmQf6cw8AvC6UPV
      rk3s3+zUORdvxB
`

const invalidYaml = `
AQICAHjdCdF7cI/3fnsYnEkQOFPNfy7fsifEtmi2Vj0qXk61IQFSxnJPmQf6cw8AvC6UPV
rk3s3+zUORdvxB
`

func Test_provider_getByPath(t *testing.T) {
	type fields struct {
		KeyPath    string
		Encryption string
	}
	type args struct {
		fileContent []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "nestedKeyWithArray",
			fields: fields{
				KeyPath: "baz.array[0].pwd",
			},
			args: args{
				fileContent: []byte(rightYaml),
			},
			wantErr: true,
		},
		{
			name: "emptyKey",
			fields: fields{
				KeyPath: "",
			},
			args: args{
				fileContent: []byte(rightYaml),
			},
			wantErr: true,
		},
		{
			name: "emptyYaml",
			fields: fields{
				KeyPath: "",
			},
			args: args{
				fileContent: []byte(invalidYaml),
			},
			wantErr: true,
		},
		{
			name: "nestedKey",
			fields: fields{
				KeyPath: "baz.password",
			},
			args: args{
				fileContent: []byte(rightYaml),
			},
			want:    "AQICAHjdCdF7cI/3fnsYnEkQOFPNfy7fsifEtmi2Vj0qXk61IQFSxnJPmQf6cw8AvC6UPVrk3s3+zUORdvxB",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &provider{
				KeyPath:    tt.fields.KeyPath,
				Encryption: tt.fields.Encryption,
			}
			got, err := p.getByPath(tt.args.fileContent)
			if (err != nil) != tt.wantErr {
				t.Errorf("getByPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getByPath() got = %v, want %v", got, tt.want)
			}
		})
	}
}
