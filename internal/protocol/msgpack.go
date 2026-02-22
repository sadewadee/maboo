package protocol

import "github.com/vmihailenco/msgpack/v5"

// MarshalMsgpack encodes a value to msgpack bytes.
func MarshalMsgpack(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}

// UnmarshalMsgpack decodes msgpack bytes into a value.
func UnmarshalMsgpack(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}
