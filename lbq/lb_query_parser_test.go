package lbq

import (
	"encoding/json"
	"os"
	"testing"
)

type JSONTest struct {
	path string
	json string
}

var tests = []JSONTest{
	/*{path: "test_files/empty.json"},
	{path: "test_files/nested_query.json"},
	{path: "test_files/inq.json"},
	{path: "test_files/and.json"},*/
	//{path: "test_files/complex.json"},
	//{path: "test_files/medium.json"},
	{path: "test_files/include.json"},
}

func TestParseFilter(t *testing.T) {
	for _, testFile := range tests {
		content, _ := os.ReadFile(testFile.path)
		lbFilter, err := ParseFilter(string(content))

		if err != nil {
			t.Fatal(err.Error())
		}

		_, err = json.MarshalIndent(lbFilter, "", "\t")
		if err != nil {
			t.Fatal(err.Error())
		}

	}
}

func BenchmarkParseFilter(b *testing.B) {
	for _, testFile := range tests {
		content, _ := os.ReadFile(testFile.path)

		testFile.json = string(content)
	}

	for _, jsonTest := range tests {
		b.Run(jsonTest.path, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = ParseFilter(jsonTest.json)
			}
		})
	}

}
