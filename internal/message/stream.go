package message

type StreamChunk interface {
	Raw() []byte
}

type FunctionCallChunk struct {
	Name          string `json:"name"`
	ArgumentsJson string `json:"arguments"`
}

type StreamHandler interface {
	HandleTextChunk(chunk []byte) error
	HandleFunctionCallStart(name string) error
	HandleFunctionCallChunk(chunk FunctionCallChunk) error
	Reset()
}
