package store

import "github.com/vmihailenco/msgpack/v5"

// Codec is an interface for encoding and decoding data.
// It is used to abstract away the underlying serialization format.
// This allows for flexibility in choosing the serialization format without changing the implementation of the store.
// The default codec is MessagePack, but other codecs can be implemented as needed. :)
type Codec interface {
	// Marshal encodes the given value into a byte slice.
	Marshal(v any) ([]byte, error)
	// Unmarshal decodes the given byte slice into the provided value.
	Unmarshal(data []byte, v any) error
}

// DefaultCodec is MessagePack.
var DefaultCodec Codec = msgpackCodec{}

type msgpackCodec struct{}

func (msgpackCodec) Marshal(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

func (msgpackCodec) Unmarshal(b []byte, v any) error {
	return msgpack.Unmarshal(b, v)
}
