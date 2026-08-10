package jsonpath

// String returns the canonical string representation of p.
func (p Path) String() string {
	if p.query == nil {
		return ""
	}
	return p.query.String()
}

// MarshalText implements encoding.TextMarshaler.
func (p Path) MarshalText() ([]byte, error) {
	if p.query == nil {
		return nil, ErrInvalidPath
	}
	return []byte(p.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (p *Path) UnmarshalText(text []byte) error {
	if p == nil {
		return ErrInvalidPath
	}
	path, err := Parse(string(text))
	if err != nil {
		return err
	}
	*p = path
	return nil
}

// Parse compiles a JSONPath expression. Returns ErrPathParse on failure.
func Parse(expr string) (Path, error) {
	p, err := NewParser()
	if err != nil {
		return Path{}, err
	}
	return p.Parse(expr)
}

// MustParse compiles a JSONPath expression. Panics on failure.
func MustParse(expr string) Path {
	path, err := Parse(expr)
	if err != nil {
		panic(err)
	}
	return path
}

// Valid reports whether expr is a syntactically valid JSONPath expression.
func Valid(expr string) bool {
	_, err := Parse(expr)
	return err == nil
}
