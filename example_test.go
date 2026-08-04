package jsonpath_test

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/agentable/go-jsonpath"
)

func ExamplePath_Select() {
	path, err := jsonpath.Parse("$.store.book[*].author")
	if err != nil {
		log.Fatal(err)
	}

	data := map[string]any{
		"store": map[string]any{
			"book": []any{
				map[string]any{"author": "Nigel Rees"},
				map[string]any{"author": "Evelyn Waugh"},
			},
		},
	}

	for author := range path.Select(data).All() {
		fmt.Println(author)
	}

	// Output:
	// Nigel Rees
	// Evelyn Waugh
}

func ExampleParseError() {
	_, err := jsonpath.Parse(" $.store")

	var parseErr *jsonpath.ParseError
	if errors.As(err, &parseErr) {
		fmt.Println(errors.Is(err, jsonpath.ErrPathParse))
		fmt.Println(parseErr.Offset)
		fmt.Println(parseErr.Reason)
	}

	// Output:
	// true
	// 0
	// leading whitespace not allowed
}

func ExampleQueryJSONLocated() {
	src := []byte(`{"store":{"book":[{"title":"Moby Dick"},{"title":"1984"}]}}`)
	path := jsonpath.MustParse("$.store.book[*].title")

	located, err := jsonpath.QueryJSONLocated(src, path)
	if err != nil {
		log.Fatal(err)
	}

	for node := range located.All() {
		fmt.Printf("%s = %v\n", node.Path, node.Value)
	}

	// Output:
	// $['store']['book'][0]['title'] = Moby Dick
	// $['store']['book'][1]['title'] = 1984
}

func hasPrefix(args []jsonpath.FunctionValue) jsonpath.Logical {
	if len(args) != 2 {
		return jsonpath.Logical(false)
	}

	value, ok := args[0].(jsonpath.Value)
	if !ok || value.IsNothing() {
		return jsonpath.Logical(false)
	}
	prefix, ok := args[1].(jsonpath.Value)
	if !ok || prefix.IsNothing() {
		return jsonpath.Logical(false)
	}

	s, ok := value.Any().(string)
	if !ok {
		return jsonpath.Logical(false)
	}
	p, ok := prefix.Any().(string)
	return jsonpath.Logical(ok && strings.HasPrefix(s, p))
}

func ExampleWithFunctions() {
	fn := jsonpath.NewLogicalFunction(
		"has_prefix",
		[]jsonpath.FuncType{jsonpath.FuncValue, jsonpath.FuncValue},
		hasPrefix,
	)
	parser, err := jsonpath.NewParser(jsonpath.WithFunctions(fn))
	if err != nil {
		log.Fatal(err)
	}
	path, err := parser.Parse(`$[?has_prefix(@.sku, "book-")]`)
	if err != nil {
		log.Fatal(err)
	}

	items := []any{
		map[string]any{"sku": "book-1"},
		map[string]any{"sku": "pen-1"},
	}

	for item := range path.Select(items).All() {
		fmt.Println(item.(map[string]any)["sku"])
	}

	// Output:
	// book-1
}
