package llm

import (
	"strings"

	"github.com/isaacphi/slop/internal/events"
)

// ToolCallArgumentParser handles incremental parsing of tool call arguments
type ToolCallArgumentParser struct {
	buffer           strings.Builder
	currentArg       string
	parsedArgs       map[string]string
	inKey            bool
	inString         bool
	escaped          bool
	keyBuffer        strings.Builder
	valueBuffer      strings.Builder
	valueBeforeChunk string // Store value buffer state before processing a new chunk
	completeJson     string // Track complete JSON
}

// NewToolCallArgumentParser creates a new parser instance
func NewToolCallArgumentParser() *ToolCallArgumentParser {
	return &ToolCallArgumentParser{
		buffer:     strings.Builder{},
		parsedArgs: make(map[string]string),
		inKey:      false, // We start expecting the opening brace
	}
}

// AddChunk processes a new chunk of JSON data and returns updates to argument values
func (p *ToolCallArgumentParser) AddChunk(chunk string) []events.Event {
	// Store the current value before processing the new chunk
	p.valueBeforeChunk = p.valueBuffer.String()

	// Append the new chunk to our buffer
	p.buffer.WriteString(chunk)
	p.completeJson += chunk

	// Updates to return
	var updates []events.Event

	// Process the buffer character by character
	data := p.buffer.String()
	p.buffer.Reset()

	for i, r := range data {
		if p.escaped {
			// Handle escaped character
			if p.inKey {
				p.keyBuffer.WriteRune(r)
			} else {
				p.valueBuffer.WriteRune(r)
			}
			p.escaped = false
			continue
		}

		switch r {
		case '\\':
			p.escaped = true
			if p.inKey {
				p.keyBuffer.WriteRune(r)
			} else {
				p.valueBuffer.WriteRune(r)
			}
		case '"':
			p.inString = !p.inString
		case ':':
			if !p.inString && p.inKey {
				// Found the separator between key and value
				keyStr := p.keyBuffer.String()
				// Extract key name without quotes
				if len(keyStr) >= 2 && strings.HasPrefix(keyStr, "\"") && strings.HasSuffix(keyStr, "\"") {
					p.currentArg = keyStr[1 : len(keyStr)-1]
				} else {
					p.currentArg = strings.TrimSpace(keyStr)
				}
				p.inKey = false
				updates = append(updates, ToolNewArgumentEvent{ArgumentName: p.currentArg})
				p.keyBuffer.Reset()
			} else {
				// Add the character to the appropriate buffer
				if p.inKey {
					p.keyBuffer.WriteRune(r)
				} else {
					p.valueBuffer.WriteRune(r)
				}
			}
		case ',':
			if !p.inString {
				// End of a key-value pair
				if !p.inKey && p.currentArg != "" {
					p.parsedArgs[p.currentArg] = p.valueBuffer.String()

					// Create an update with the completed value
					if p.valueBuffer.Len() > 0 {
						valueStr := p.valueBuffer.String()
						update := ToolArgumentChunkEvent{
							ArgumentName: p.currentArg,
							Chunk:        valueStr[len(p.valueBeforeChunk):],
						}
						updates = append(updates, update)
					}

					p.valueBuffer.Reset()
				}
				p.inKey = true
				p.currentArg = ""
			} else {
				// Add the character to the appropriate buffer
				if p.inKey {
					p.keyBuffer.WriteRune(r)
				} else {
					p.valueBuffer.WriteRune(r)
				}
			}
		case '{':
			// Start of object
			if p.keyBuffer.Len() == 0 && p.valueBuffer.Len() == 0 {
				// This is the opening brace of the entire object
				p.inKey = true
			} else {
				// Add the character to the appropriate buffer
				if p.inKey {
					p.keyBuffer.WriteRune(r)
				} else {
					p.valueBuffer.WriteRune(r)
				}
			}
		case '}':
			// End of object - store the final value
			if !p.inString && !p.inKey && p.currentArg != "" {
				p.parsedArgs[p.currentArg] = p.valueBuffer.String()
			} else {
				if p.inKey {
					p.keyBuffer.WriteRune(r)
				} else {
					p.valueBuffer.WriteRune(r)
				}
			}
		default:
			// Regular character
			if p.inKey {
				p.keyBuffer.WriteRune(r)
			} else if p.inString {
				p.valueBuffer.WriteRune(r)
			}
		}

		// If we reached the end of the buffer and there's more to process,
		// put the rest back into the main buffer
		if i == len(data)-1 && len(data) > 1 {
			p.buffer.WriteString(data[i+1:])
		}
	}

	// Check if we have an incremental update for the current value
	currentValue := p.valueBuffer.String()
	if !p.inKey && p.currentArg != "" && len(currentValue) > len(p.valueBeforeChunk) {
		// Only create an update if there's new content
		incrementalUpdate := currentValue[len(p.valueBeforeChunk):]
		if len(incrementalUpdate) > 0 {
			update := ToolArgumentChunkEvent{
				ArgumentName: p.currentArg,
				Chunk:        incrementalUpdate,
			}
			updates = append(updates, update)
		}
	}

	return updates
}

// GetCurrentArgName returns the name of the argument currently being parsed
func (p *ToolCallArgumentParser) GetCurrentArgName() string {
	return p.currentArg
}

// GetCurrentValue returns the value of the current argument as it's being parsed
func (p *ToolCallArgumentParser) GetCurrentValue() string {
	return p.valueBuffer.String()
}

// GetAllData returns the complete JSON received so far
func (p *ToolCallArgumentParser) GetAllData() string {
	return p.completeJson
}

// GetParsedArgs returns all completely parsed arguments
func (p *ToolCallArgumentParser) GetParsedArgs() map[string]string {
	return p.parsedArgs
}

// Reset clears the parser state
func (p *ToolCallArgumentParser) Reset() {
	p.buffer.Reset()
	p.keyBuffer.Reset()
	p.valueBuffer.Reset()
	p.valueBeforeChunk = ""
	p.currentArg = ""
	p.parsedArgs = make(map[string]string)
	p.inKey = false
	p.inString = false
	p.escaped = false
	p.completeJson = ""
}
