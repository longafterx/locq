package core

import "testing"

func testRunState(t testing.TB, opts *Options) *runState {
	t.Helper()
	config, err := opts.prepare()
	if err != nil {
		t.Fatal(err)
	}
	if opts == nil {
		return newRunState(config, nil, nil)
	}
	return newRunState(config, opts.Diagnostics, nil)
}

// Adapt independent semantic assertions to the public synchronous event API.
func observeKinds(code, comment, blank func(string)) func(LineEvent) {
	return func(event LineEvent) {
		callback := code
		switch event.Kind {
		case CommentLine:
			callback = comment
		case BlankLine:
			callback = blank
		}
		if callback != nil {
			callback(event.Text)
		}
	}
}
