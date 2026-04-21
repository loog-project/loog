package store

import (
	"bytes"
	"sync"

	"github.com/vmihailenco/msgpack/v5"
)

// Codec is an interface for encoding and decoding data.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// DefaultCodec is a pooled MessagePack codec that reuses encoder/decoder
// objects and their underlying buffers across calls.
var DefaultCodec Codec = &pooledMsgpackCodec{}

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

type pooledMsgpackCodec struct{}

func (pooledMsgpackCodec) Marshal(v any) ([]byte, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	enc := msgpack.GetEncoder()
	enc.Reset(buf)

	err := enc.Encode(v)
	msgpack.PutEncoder(enc)

	if err != nil {
		bufPool.Put(buf)
		return nil, err
	}

	// Copy out so the pooled buffer can be reused immediately.
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	bufPool.Put(buf)
	return out, nil
}

func (pooledMsgpackCodec) Unmarshal(data []byte, v any) error {
	dec := msgpack.GetDecoder()
	dec.Reset(bytes.NewReader(data))
	err := dec.Decode(v)
	msgpack.PutDecoder(dec)
	return err
}
