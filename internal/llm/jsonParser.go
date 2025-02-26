package llm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/isaacphi/slop/internal/domain"
)

// Parser state constants
type ParserState int

const (
	StateObjectStart ParserState = iota
	StateObjectKey
	StateObjectColon
	StateObjectValue
	StateObjectComma
	StateArrayStart
	StateArrayValue
	StateArrayComma
	StateString
	StateStringEscape
	StateNumber
	StateTrue
	StateFalse
	StateNull
)

// StateContext tracks the current parsing context
type StateContext struct {
	State      ParserState
	Property   *domain.Property
	Key        string
	ArrayIndex int // For arrays
}

type IncrementalJsonParser struct {
	completeString strings.Builder
	schema         *domain.Parameters

	// Parser state tracking
	stateStack []StateContext
	pathStack  []string
	currentKey strings.Builder
	literalPos int // Position within true/false/null literals

	// Value accumulation
	valueBuffer map[string]*strings.Builder
	currentPath string

	lastProcessed int // Position of last processed character
}

// NewIncrementalJsonParser creates a new parser with the given schema
func NewIncrementalJsonParser(schema *domain.Parameters) *IncrementalJsonParser {
	// Create root property from schema
	rootProperty := &domain.Property{
		Type:       schema.Type,
		Properties: schema.Properties,
		Required:   schema.Required,
	}

	parser := &IncrementalJsonParser{
		schema: schema,
		stateStack: []StateContext{
			{
				State:    StateObjectStart,
				Property: rootProperty,
			},
		},
		pathStack:   make([]string, 0),
		valueBuffer: make(map[string]*strings.Builder),
	}

	return parser
}

// getCurrentPath returns the current JSON path
func (p *IncrementalJsonParser) getCurrentPath() string {
	if len(p.pathStack) == 0 {
		return ""
	}

	var result strings.Builder
	for i, component := range p.pathStack {
		if i > 0 && !strings.HasPrefix(component, "[") {
			result.WriteString(".")
		}
		result.WriteString(component)
	}

	return result.String()
}

// flushBuffers converts all accumulated value buffers to events
func (p *IncrementalJsonParser) flushBuffers() []JsonUpdateEvent {
	events := make([]JsonUpdateEvent, 0, len(p.valueBuffer))

	for path, buffer := range p.valueBuffer {
		if buffer.Len() > 0 {
			events = append(events, JsonUpdateEvent{
				Key:        path,
				ValueChunk: buffer.String(),
			})
			buffer.Reset()
		}
	}

	// Clear map after flushing
	p.valueBuffer = make(map[string]*strings.Builder)

	return events
}

func (p *IncrementalJsonParser) appendToValueBuffer(path, value string) {
	if _, exists := p.valueBuffer[path]; !exists {
		p.valueBuffer[path] = &strings.Builder{} // Store a pointer
	}
	p.valueBuffer[path].WriteString(value)
}

// ProcessChunk processes a chunk of JSON and returns update events
func (p *IncrementalJsonParser) ProcessChunk(chunk string) ([]JsonUpdateEvent, error) {
	p.completeString.WriteString(chunk)
	fullStr := p.completeString.String()

	events := make([]JsonUpdateEvent, 0)

	for i := p.lastProcessed; i < len(fullStr); i++ {
		char := fullStr[i]

		// Skip whitespace in most states except inside strings
		if (char == ' ' || char == '\t' || char == '\n' || char == '\r') &&
			p.stateStack[len(p.stateStack)-1].State != StateString &&
			p.stateStack[len(p.stateStack)-1].State != StateStringEscape {
			continue
		}

		// Get current context from top of stack
		if len(p.stateStack) == 0 {
			flushEvents := p.flushBuffers()
			events = append(events, flushEvents...)
			return events, errors.New("invalid JSON: parser state stack is empty")
		}
		ctx := &p.stateStack[len(p.stateStack)-1]

		// Calculate current path
		newPath := p.getCurrentPath()
		if p.currentPath != newPath {
			// Path changed, flush buffers
			flushEvents := p.flushBuffers()
			events = append(events, flushEvents...)
			p.currentPath = newPath
		}

		switch ctx.State {
		case StateObjectStart:
			if char == '{' {
				// Start of object - stay in this state
			} else if char == '"' {
				// Start of key
				ctx.State = StateObjectKey
				p.currentKey.Reset()
			} else if char == '}' {
				// End of empty object
				if len(p.stateStack) > 1 {
					// Pop current state
					p.stateStack = p.stateStack[:len(p.stateStack)-1]
					if len(p.pathStack) > 0 {
						p.pathStack = p.pathStack[:len(p.pathStack)-1]
					}

					// Update parent state
					if len(p.stateStack) == 0 {
						flushEvents := p.flushBuffers()
						events = append(events, flushEvents...)
						return events, errors.New("invalid JSON: unexpected closing brace")
					}
					parentCtx := &p.stateStack[len(p.stateStack)-1]
					if parentCtx.State == StateObjectValue {
						parentCtx.State = StateObjectComma
					} else if parentCtx.State == StateArrayValue {
						parentCtx.State = StateArrayComma
						parentCtx.ArrayIndex++
					}
				}
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: unexpected character '%c' at object start", char)
			}

		case StateObjectKey:
			if char == '\\' {
				// Escape sequence in key
				p.stateStack = append(p.stateStack, StateContext{
					State: StateStringEscape,
				})
			} else if char == '"' {
				// End of key
				ctx.State = StateObjectColon
				keyStr := p.currentKey.String()
				ctx.Key = keyStr

				// Validate key against schema
				if ctx.Property != nil && ctx.Property.Type == "object" && ctx.Property.Properties != nil {
					if prop, exists := ctx.Property.Properties[keyStr]; exists {
						// Valid key according to schema
						ctx.Property = &prop
					} else {
						// Unknown key, not in schema
						ctx.Property = nil
					}
				} else {
					// We're parsing an object but no schema for its properties
					ctx.Property = nil
				}

				// Add key to path stack
				p.pathStack = append(p.pathStack, keyStr)
			} else {
				// Regular character in key
				p.currentKey.WriteByte(char)
			}

		case StateObjectColon:
			if char == ':' {
				// Move to value state
				ctx.State = StateObjectValue
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: expected ':' but got '%c'", char)
			}

		case StateObjectValue:
			// Determine what kind of value follows
			if char == '"' {
				// String value
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateString,
					Property: ctx.Property,
				})
			} else if char == '{' {
				// Nested object
				var nextProp *domain.Property
				if ctx.Property != nil && ctx.Property.Type == "object" {
					nextProp = ctx.Property
				}
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateObjectStart,
					Property: nextProp,
				})
			} else if char == '[' {
				// Array
				var itemsProp *domain.Property
				if ctx.Property != nil && ctx.Property.Type == "array" && ctx.Property.Items != nil {
					itemsProp = ctx.Property.Items
				}
				p.stateStack = append(p.stateStack, StateContext{
					State:      StateArrayStart,
					Property:   itemsProp,
					ArrayIndex: 0,
				})
			} else if char == 't' {
				// true
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateTrue,
					Property: ctx.Property,
				})
				p.literalPos = 1 // Already processed 't'

				p.appendToValueBuffer(p.getCurrentPath(), "t")
			} else if char == 'f' {
				// false
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateFalse,
					Property: ctx.Property,
				})
				p.literalPos = 1 // Already processed 'f'

				p.appendToValueBuffer(p.getCurrentPath(), "f")
			} else if char == 'n' {
				// null
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateNull,
					Property: ctx.Property,
				})
				p.literalPos = 1 // Already processed 'n'

				p.appendToValueBuffer(p.getCurrentPath(), "n")
			} else if char == '-' || (char >= '0' && char <= '9') {
				// Number
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateNumber,
					Property: ctx.Property,
				})

				p.appendToValueBuffer(p.getCurrentPath(), string(char))
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: unexpected character '%c' in object value", char)
			}

		case StateObjectComma:
			if char == ',' {
				// Pop the last path segment
				if len(p.pathStack) > 0 {
					p.pathStack = p.pathStack[:len(p.pathStack)-1]
				}

				// Reset to parent's property for next key-value pair
				if len(p.stateStack) > 1 {
					parentCtx := p.stateStack[len(p.stateStack)-2]
					ctx.Property = parentCtx.Property
				}

				// Expect next key-value pair
				ctx.State = StateObjectStart
			} else if char == '}' {
				// End of object
				// Pop the current state
				p.stateStack = p.stateStack[:len(p.stateStack)-1]

				// Pop the last path segment
				if len(p.pathStack) > 0 {
					p.pathStack = p.pathStack[:len(p.pathStack)-1]
				}

				// Update parent state
				if len(p.stateStack) > 0 {
					parentCtx := &p.stateStack[len(p.stateStack)-1]
					if parentCtx.State == StateObjectValue {
						parentCtx.State = StateObjectComma
					} else if parentCtx.State == StateArrayValue {
						parentCtx.State = StateArrayComma
						parentCtx.ArrayIndex++
					}
				}
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: expected ',' or '}' but got '%c'", char)
			}

		case StateArrayStart:
			if char == '[' {
				// Start of array - stay in this state
			} else if char == ']' {
				// Empty array
				// Pop the current state
				p.stateStack = p.stateStack[:len(p.stateStack)-1]

				// Update parent state
				if len(p.stateStack) > 0 {
					parentCtx := &p.stateStack[len(p.stateStack)-1]
					if parentCtx.State == StateObjectValue {
						parentCtx.State = StateObjectComma
					} else if parentCtx.State == StateArrayValue {
						parentCtx.State = StateArrayComma
						parentCtx.ArrayIndex++
					}
				}
			} else {
				// Start of a value in the array
				ctx.State = StateArrayValue

				// Prepare path for array element
				arrayIndex := fmt.Sprintf("[%d]", ctx.ArrayIndex)
				p.pathStack = append(p.pathStack, arrayIndex)

				// Reprocess this character
				i--
			}

		case StateArrayValue:
			// Determine what kind of value follows in the array
			if char == '"' {
				// String value
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateString,
					Property: ctx.Property,
				})
			} else if char == '{' {
				// Nested object in array
				var nextProp *domain.Property
				if ctx.Property != nil && ctx.Property.Type == "object" {
					nextProp = ctx.Property
				}
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateObjectStart,
					Property: nextProp,
				})
			} else if char == '[' {
				// Nested array
				var itemsProp *domain.Property
				if ctx.Property != nil && ctx.Property.Type == "array" && ctx.Property.Items != nil {
					itemsProp = ctx.Property.Items
				}
				p.stateStack = append(p.stateStack, StateContext{
					State:      StateArrayStart,
					Property:   itemsProp,
					ArrayIndex: 0,
				})
			} else if char == ']' {
				// End of array after a value
				// Pop the array element path
				if len(p.pathStack) > 0 {
					p.pathStack = p.pathStack[:len(p.pathStack)-1]
				}

				// Pop the current state
				p.stateStack = p.stateStack[:len(p.stateStack)-1]

				// Update parent state
				if len(p.stateStack) > 0 {
					parentCtx := &p.stateStack[len(p.stateStack)-1]
					if parentCtx.State == StateObjectValue {
						parentCtx.State = StateObjectComma
					} else if parentCtx.State == StateArrayValue {
						parentCtx.State = StateArrayComma
						parentCtx.ArrayIndex++
					}
				}
			} else if char == 't' {
				// true
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateTrue,
					Property: ctx.Property,
				})
				p.literalPos = 1 // Already processed 't'

				p.appendToValueBuffer(p.getCurrentPath(), "t")
			} else if char == 'f' {
				// false
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateFalse,
					Property: ctx.Property,
				})
				p.literalPos = 1 // Already processed 'f'

				p.appendToValueBuffer(p.getCurrentPath(), "f")
			} else if char == 'n' {
				// null
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateNull,
					Property: ctx.Property,
				})
				p.literalPos = 1 // Already processed 'n'

				p.appendToValueBuffer(p.getCurrentPath(), "n")
			} else if char == '-' || (char >= '0' && char <= '9') {
				// Number
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateNumber,
					Property: ctx.Property,
				})

				p.appendToValueBuffer(p.getCurrentPath(), string(char))
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: unexpected character '%c' in array value", char)
			}

		case StateArrayComma:
			if char == ',' {
				// Pop the array element path
				if len(p.pathStack) > 0 {
					p.pathStack = p.pathStack[:len(p.pathStack)-1]
				}

				// Stay in array context, expect next value
				ctx.State = StateArrayValue

				// Prepare path for next array element
				arrayIndex := fmt.Sprintf("[%d]", ctx.ArrayIndex)
				p.pathStack = append(p.pathStack, arrayIndex)
			} else if char == ']' {
				// End of array
				// Pop the current state
				p.stateStack = p.stateStack[:len(p.stateStack)-1]

				// Update parent state
				if len(p.stateStack) > 0 {
					parentCtx := &p.stateStack[len(p.stateStack)-1]
					if parentCtx.State == StateObjectValue {
						parentCtx.State = StateObjectComma
					} else if parentCtx.State == StateArrayValue {
						parentCtx.State = StateArrayComma
						parentCtx.ArrayIndex++
					}
				}
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: expected ',' or ']' but got '%c'", char)
			}

		case StateString:
			if char == '\\' {
				// Escape sequence
				p.stateStack = append(p.stateStack, StateContext{
					State:    StateStringEscape,
					Property: ctx.Property,
				})
			} else if char == '"' {
				// End of string
				// Pop the string state
				p.stateStack = p.stateStack[:len(p.stateStack)-1]

				// Update parent state
				if len(p.stateStack) > 0 {
					parentCtx := &p.stateStack[len(p.stateStack)-1]
					if parentCtx.State == StateObjectValue {
						parentCtx.State = StateObjectComma
					} else if parentCtx.State == StateArrayValue {
						parentCtx.State = StateArrayComma
					}
				}

				// Flush accumulated value
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
			} else {
				// Regular character in string
				p.appendToValueBuffer(p.getCurrentPath(), string(char))
			}

		case StateStringEscape:
			// Handle escape sequence
			var escaped string
			switch char {
			case 'n':
				escaped = "\n"
			case 'r':
				escaped = "\r"
			case 't':
				escaped = "\t"
			case 'b':
				escaped = "\b"
			case 'f':
				escaped = "\f"
			case '\\':
				escaped = "\\"
			case '/':
				escaped = "/"
			case '"':
				escaped = "\""
			case 'u':
				// Unicode escape - simplified handling
				// A full implementation would parse the 4 hex digits
				escaped = "\\u"
			default:
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: invalid escape sequence '\\%c'", char)
			}

			// If we're in a key, add to key builder, otherwise append to value buffer
			if len(p.stateStack) >= 2 && p.stateStack[len(p.stateStack)-2].State == StateObjectKey {
				p.currentKey.WriteString(escaped)
			} else {
				p.appendToValueBuffer(p.getCurrentPath(), escaped)
			}

			// Pop back to string state
			p.stateStack = p.stateStack[:len(p.stateStack)-1]

		case StateNumber:
			if (char >= '0' && char <= '9') || char == '.' || char == 'e' || char == 'E' || char == '+' || char == '-' {
				// Continue number
				p.appendToValueBuffer(p.getCurrentPath(), string(char))
			} else {
				// End of number
				// Pop the number state
				p.stateStack = p.stateStack[:len(p.stateStack)-1]

				// Update parent state
				if len(p.stateStack) > 0 {
					parentCtx := &p.stateStack[len(p.stateStack)-1]
					if parentCtx.State == StateObjectValue {
						parentCtx.State = StateObjectComma
					} else if parentCtx.State == StateArrayValue {
						parentCtx.State = StateArrayComma
					}
				}

				// Flush accumulated value
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)

				// Reprocess this character
				i--
			}

		case StateTrue:
			expected := "true"
			if char == expected[p.literalPos] {
				p.appendToValueBuffer(p.getCurrentPath(), string(char))

				p.literalPos++
				if p.literalPos == len(expected) {
					// End of true literal
					// Pop the true state
					p.stateStack = p.stateStack[:len(p.stateStack)-1]

					// Update parent state
					if len(p.stateStack) > 0 {
						parentCtx := &p.stateStack[len(p.stateStack)-1]
						if parentCtx.State == StateObjectValue {
							parentCtx.State = StateObjectComma
						} else if parentCtx.State == StateArrayValue {
							parentCtx.State = StateArrayComma
						}
					}

					// Flush accumulated value
					flushEvents := p.flushBuffers()
					events = append(events, flushEvents...)
				}
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: expected 'true' but got invalid character '%c'", char)
			}

		case StateFalse:
			expected := "false"
			if char == expected[p.literalPos] {
				p.appendToValueBuffer(p.getCurrentPath(), string(char))

				p.literalPos++
				if p.literalPos == len(expected) {
					// End of false literal
					// Pop the false state
					p.stateStack = p.stateStack[:len(p.stateStack)-1]

					// Update parent state
					if len(p.stateStack) > 0 {
						parentCtx := &p.stateStack[len(p.stateStack)-1]
						if parentCtx.State == StateObjectValue {
							parentCtx.State = StateObjectComma
						} else if parentCtx.State == StateArrayValue {
							parentCtx.State = StateArrayComma
						}
					}

					// Flush accumulated value
					flushEvents := p.flushBuffers()
					events = append(events, flushEvents...)
				}
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: expected 'false' but got invalid character '%c'", char)
			}

		case StateNull:
			expected := "null"
			if char == expected[p.literalPos] {
				p.appendToValueBuffer(p.getCurrentPath(), string(char))

				p.literalPos++
				if p.literalPos == len(expected) {
					// End of null literal
					// Pop the null state
					p.stateStack = p.stateStack[:len(p.stateStack)-1]

					// Update parent state
					if len(p.stateStack) > 0 {
						parentCtx := &p.stateStack[len(p.stateStack)-1]
						if parentCtx.State == StateObjectValue {
							parentCtx.State = StateObjectComma
						} else if parentCtx.State == StateArrayValue {
							parentCtx.State = StateArrayComma
						}
					}

					// Flush accumulated value
					flushEvents := p.flushBuffers()
					events = append(events, flushEvents...)
				}
			} else {
				flushEvents := p.flushBuffers()
				events = append(events, flushEvents...)
				return events, fmt.Errorf("invalid JSON: expected 'null' but got invalid character '%c'", char)
			}
		}
	}

	// Update the last processed position
	p.lastProcessed = len(fullStr)

	// Only flush buffers at end of chunk if we've accumulated values
	if len(p.valueBuffer) > 0 {
		flushEvents := p.flushBuffers()
		events = append(events, flushEvents...)
	}

	return events, nil
}
