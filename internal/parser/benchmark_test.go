package parser

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	var large strings.Builder
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&large, "KEY_%05d=synthetic-value\n", i)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{"typical", []byte("# synthetic settings\nHOST=localhost\nPORT=3000\nDEBUG=false\nTOKEN=example\n")},
		{"large_valid", []byte(large.String())},
		{"newline_dense_10MiB", bytes.Repeat([]byte{'\n'}, MaxSize)},
		{"long_line_10MiB", append([]byte("KEY="), bytes.Repeat([]byte{'x'}, MaxSize-4)...)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Parse(bytes.NewReader(tc.data), Reference); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
