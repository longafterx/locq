package main

import "testing"

func TestNormalizePath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, root, input, want string
		invalid                 bool
	}{
		{name: "relative", root: "/repo", input: "./src/a.go", want: "src/a.go"},
		{name: "absolute", root: "/repo", input: "/repo/src/a.go", want: "src/a.go"},
		{name: "windows relative", root: "/repo", input: `src\a.go`, want: "src/a.go"},
		{name: "windows absolute", root: `C:\repo`, input: `C:\repo\a.go`, want: "a.go"},
		{name: "parent", root: "/repo", input: "../a.go", invalid: true},
		{name: "absolute escape", root: "/repo", input: "/other/a.go", invalid: true},
		{name: "drive escape", root: "/repo", input: `C:\other\a.go`, invalid: true},
		{name: "root", root: "/repo", input: ".", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizePath(tc.root, tc.input)
			if (err != nil) != tc.invalid || got != tc.want {
				t.Fatalf("got %q/%v want %q invalid=%t", got, err, tc.want, tc.invalid)
			}
		})
	}
}

// JSON fixtures remain verbatim on one line so invalid and duplicate fields are visible.
func TestDecodeSnapshot(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, tool, input, language string
		invalid                     bool
		embedded                    bool
		inaccurate                  bool
	}{
		{name: "locq", tool: "locq", language: "Go", input: `{"files":[{"name":"./a.go","language":"Go","code":1,"comment":2,"blank":3}]}`},
		{name: "cloc", tool: "cloc", language: "C Header", input: `{"header":{},"SUM":{},"a.go":{"language":"C/C++ Header","code":1,"comment":2,"blank":3}}`},
		{name: "tokei", tool: "tokei", language: "Go", input: `{"Go":{"reports":[{"name":"a.go","stats":{"code":1,"comments":2,"blanks":3,"blobs":{}}}]},"Total":{}}`},
		{name: "blobs preserved", tool: "tokei", language: "HTML", embedded: true, input: `{"HTML":{"reports":[{"name":"a.go","stats":{"code":1,"comments":2,"blanks":3,"blobs":{"JavaScript":{"code":4,"comments":5,"blanks":6}}}}]}}`},
		{name: "inaccurate", tool: "tokei", language: "Go", inaccurate: true, input: `{"Go":{"inaccurate":true,"reports":[{"name":"a.go","stats":{"code":1,"comments":2,"blanks":3}}]}}`},
		{name: "aggregate rejected", tool: "locq", input: `{"languages":[]}`, invalid: true},
		{name: "null files", tool: "locq", input: `{"files":null}`, invalid: true},
		{name: "null reports", tool: "tokei", input: `{"Go":{"reports":null}}`, invalid: true},
		{name: "total inaccurate", tool: "tokei", language: "Go", inaccurate: true,
			input: `{"Go":{"reports":[{"name":"a.go","stats":{"code":1,"comments":2,"blanks":3}}]},"Total":{"inaccurate":true}}`},
		{name: "missing count", tool: "locq", input: `{"files":[{"name":"a.go","language":"Go","code":1}]}`, invalid: true},
		{name: "null count", tool: "cloc", input: `{"a.go":{"language":"Go","code":null,"comment":2,"blank":3}}`, invalid: true},
		{name: "negative count", tool: "locq", input: `{"files":[{"name":"a.go","language":"Go","code":-1,"comment":2,"blank":3}]}`, invalid: true},
		{name: "fraction count", tool: "cloc", input: `{"a.go":{"language":"Go","code":0.5,"comment":2,"blank":3}}`, invalid: true},
		{name: "duplicate paths", tool: "locq", input: `{"files":[{"name":"a.go","language":"Go","code":1,"comment":2,"blank":3},{"name":"./a.go","language":"Go","code":1,"comment":2,"blank":3}]}`, invalid: true},
		{name: "path escape", tool: "cloc", input: `{"../a.go":{"language":"Go","code":1,"comment":2,"blank":3}}`, invalid: true},
		{name: "invalid JSON", tool: "tokei", input: `{"Go":`, invalid: true},
		{name: "null JSON", tool: "tokei", input: `null`, invalid: true},
		{name: "trailing JSON", tool: "locq", input: `{"files":[]} {}`, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeSnapshot(tc.tool, "/repo", []byte(tc.input))
			if (err != nil) != tc.invalid {
				t.Fatalf("error=%v, invalid=%t", err, tc.invalid)
			}
			if tc.invalid {
				return
			}
			file := got.Files["a.go"]
			if file.Language != tc.language || file.Counts != (counts{Code: 1, Comment: 2, Blank: 3}) {
				t.Fatalf("normalized file: %+v", file)
			}
			if (len(file.Embedded) > 0) != tc.embedded {
				t.Fatalf("embedded language lost: %+v", file)
			}
			if got.Complete == tc.inaccurate {
				t.Fatal("incomplete flag lost")
			}
		})
	}
}
