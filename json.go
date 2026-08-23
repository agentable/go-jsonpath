package jsonpath

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
)

// QueryJSON unmarshals src, preserving numbers as [jsontext.Value], and evaluates path.
func QueryJSON(src []byte, path Path) (NodeList, error) {
	if path.query == nil {
		return nil, ErrInvalidPath
	}
	v, err := unmarshalJSON(src)
	if err != nil {
		return nil, err
	}
	return path.Select(v), nil
}

// QueryJSONRead unmarshals JSON from r and evaluates path.
func QueryJSONRead(r io.Reader, path Path) (NodeList, error) {
	if path.query == nil {
		return nil, ErrInvalidPath
	}
	v, err := unmarshalJSONRead(r)
	if err != nil {
		return nil, err
	}
	return path.Select(v), nil
}

// QueryJSONLocated unmarshals src and returns matched nodes with normalized paths.
func QueryJSONLocated(src []byte, path Path) (LocatedNodeList, error) {
	if path.query == nil {
		return nil, ErrInvalidPath
	}
	v, err := unmarshalJSON(src)
	if err != nil {
		return nil, err
	}
	return path.SelectLocated(v), nil
}

// QueryJSONReadLocated unmarshals JSON from r and returns located matches.
func QueryJSONReadLocated(r io.Reader, path Path) (LocatedNodeList, error) {
	if path.query == nil {
		return nil, ErrInvalidPath
	}
	v, err := unmarshalJSONRead(r)
	if err != nil {
		return nil, err
	}
	return path.SelectLocated(v), nil
}

func unmarshalJSON(src []byte) (any, error) {
	var v any
	if err := json.Unmarshal(src, &v, preserveJSONNumbers); err != nil {
		return nil, errors.Join(ErrUnmarshal, err)
	}
	return v, nil
}

func unmarshalJSONRead(r io.Reader) (any, error) {
	var v any
	if err := json.UnmarshalRead(r, &v, preserveJSONNumbers); err != nil {
		return nil, errors.Join(ErrUnmarshal, err)
	}
	return v, nil
}

var preserveJSONNumbers = json.WithUnmarshalers(json.UnmarshalFromFunc(
	func(dec *jsontext.Decoder, dst *any) error {
		if dec.PeekKind() != '0' {
			return errors.ErrUnsupported
		}
		src, err := dec.ReadValue()
		if err != nil {
			return err
		}
		*dst = src.Clone()
		return nil
	},
))
