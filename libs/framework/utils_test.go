package framework

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr string
	}{
		{
			name:  "simple flags",
			input: `-input abc-def -workers -1`,
			want:  []string{"-input", "abc-def", "-workers", "-1"},
		},
		{
			name:  "quoted spaces",
			input: `-input "C:\data\my dir\file.geojson" -output '/tmp/out dir'`,
			want:  []string{"-input", `C:\data\my dir\file.geojson`, "-output", "/tmp/out dir"},
		},
		{
			name:  "escaped quotes",
			input: `-name "a\"b" -remark 'it\'s ok'`,
			want:  []string{"-name", `a"b`, "-remark", "it's ok"},
		},
		{
			name:  "empty string value",
			input: `-input "" -label value`,
			want:  []string{"-input", "", "-label", "value"},
		},
		{
			name:  "escaped spaces outside quotes",
			input: `-input C:\my\ path\ with\ spaces\data.txt`,
			want:  []string{"-input", `C:\my path with spaces\data.txt`},
		},
		{
			name:  "windows path ending with slash before next flag",
			input: `-input /production/file -output C:\Users\zhangzijiang\Desktop\260602新分类\ -client 10.11.5.136:50070 -user hdfs`,
			want: []string{
				"-input",
				"/production/file",
				"-output",
				`C:\Users\zhangzijiang\Desktop\260602新分类\`,
				"-client",
				"10.11.5.136:50070",
				"-user",
				"hdfs",
			},
		},
		{
			name:    "unterminated double quote",
			input:   `-input "unterminated`,
			wantErr: "双引号未闭合",
		},
		{
			name:    "unterminated single quote",
			input:   `-input 'unterminated`,
			wantErr: "单引号未闭合",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseArgs(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if err != nil && !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseArgs returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseArgs mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}
