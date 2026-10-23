package lexertest

// LegacyCases fixes current host-language behavior without enabling embeddings.
func LegacyCases() []Case {
	return []Case{
		{Name: "legacy/HTML/script stays host", Language: "HTML", Extension: "html",
			Source: "<script>\n// script text\n</script>\n<!-- real -->\n", Kinds: "CMCM"},
		{Name: "legacy/Vue/script stays host", Language: "Vue", Extension: "vue",
			Source: "<script>\n// script text\n</script>\n<!-- real -->\n", Kinds: "CCCM"},
		{Name: "legacy/Svelte/script stays host", Language: "Svelte", Extension: "svelte",
			Source: "<script>\n// script text\n</script>\n<!-- real -->\n", Kinds: "CMCM"},
		{Name: "legacy/Markdown/fence stays host", Language: "Markdown", Extension: "md",
			Source: "text\n\n```go\n// fenced text\n```\n", Kinds: "CBCCC"},
		{Name: "legacy/JSX/ordinary comments", Language: "JSX", Extension: "jsx",
			Source: "const element = <div/>;\n// real\n", Kinds: "CM"},
		{Name: "legacy/Plain Text/prose is code", Language: "Plain Text", Extension: "txt",
			Source: "some text\n\n// still text\n", Kinds: "CBC"},
		{Name: "legacy/notebook/JSON stays host", Language: "Jupyter Notebook", Extension: "ipynb",
			Source: "{\"cells\": [{\"source\": [\"# text\"]}]}\n", Kinds: "C"},
	}
}
