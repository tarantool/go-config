package collectors

// rawBytesMarshaller implements marshaller.TypedMarshaller[[]byte] by passing
// bytes through without any encoding. Use this when the bytes already contain
// serialized content (e.g., pre-built YAML) to avoid double-encoding.
type rawBytesMarshaller struct{}

// Marshal returns the data as-is without encoding.
func (rawBytesMarshaller) Marshal(data []byte) ([]byte, error) {
	return data, nil
}

// Unmarshal returns the data as-is without decoding.
func (rawBytesMarshaller) Unmarshal(data []byte) ([]byte, error) {
	return data, nil
}
