package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/events"
	"github.com/isaacphi/slop/internal/llm"
)

// SendMessageStream sends a message through the Agent and returns a stream of events
func (a *Agent) SendMessageStream(ctx context.Context, opts SendMessageOptions) AgentStream {
	eventsChan := make(chan events.Event)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer close(eventsChan)

		// Validation
		thread, err := a.repository.GetThread(ctx, opts.ThreadID)
		if err != nil {
			eventsChan <- &events.ErrorEvent{
				Error: fmt.Errorf("failed to get thread: %w", err),
			}
			return
		}

		if opts.Role == domain.RoleAssistant {
			eventsChan <- &events.ErrorEvent{
				Error: fmt.Errorf("cannot send message with role Assistant"),
			}
			return
		}

		// If no parent specified, get the most recent message in thread
		if opts.ParentID == nil {
			messages, err := a.repository.GetMessages(ctx, thread.ID, nil, false)
			if err != nil {
				eventsChan <- &events.ErrorEvent{
					Error: fmt.Errorf("failed to get messages: %w", err),
				}
				return
			}
			if len(messages) > 0 {
				lastMsg := messages[len(messages)-1]
				opts.ParentID = &lastMsg.ID
			}
		}

		// Get conversation history for context
		history, err := a.repository.GetMessages(ctx, thread.ID, opts.ParentID, false)
		if err != nil {
			eventsChan <- &events.ErrorEvent{
				Error: fmt.Errorf("failed to get conversation history: %w", err),
			}
			return
		}

		// Build system message
		systemMessage, err := a.buildSystemMessage(systemMessageOpts{
			messageContent: opts.Content,
			history:        history,
		})
		if err != nil {
			eventsChan <- &events.ErrorEvent{
				Error: fmt.Errorf("failed to build system message: %w", err),
			}
			return
		}

		// Create message
		userMsg := &domain.Message{
			ThreadID: opts.ThreadID,
			ParentID: opts.ParentID,
			Role:     opts.Role,
			Content:  opts.Content,
		}

		if err := a.repository.AddMessageToThread(ctx, opts.ThreadID, userMsg); err != nil {
			eventsChan <- &events.ErrorEvent{
				Error: fmt.Errorf("failed to add message to thread: %w", err),
			}
			return
		}

		// Send user message event
		eventsChan <- &NewMessageEvent{
			Message: userMsg,
		}

		// Get AI response
		generateOptions := llm.GenerateContentOptions{
			Preset:        a.preset,
			Content:       opts.Content,
			SystemMessage: systemMessage,
			History:       history,
			Tools:         flattenTools(a.tools),
		}

		// Get LLM stream
		llmStream := llm.GenerateContentStream(ctx, generateOptions)

		// Track assistant response for saving
		var toolCalls []llm.ToolCall

		// Forward LLM events to agent stream
		for {
			select {
			case <-ctx.Done():
				eventsChan <- &events.ErrorEvent{Error: ctx.Err()}
				return

			case event, ok := <-llmStream.Events:
				if !ok {
					return
				}

				// Handle event and collect response data
				switch e := event.(type) {
				case *llm.TextEvent:
					// Forward the event
					eventsChan <- e

				case *llm.ToolCallEvent:
					// Forward the event
					eventsChan <- e

				case *llm.MessageCompleteEvent:
					// Create and save AI message
					aiMsg := &domain.Message{
						ThreadID:  opts.ThreadID,
						ParentID:  &userMsg.ID,
						Role:      domain.RoleAssistant,
						Content:   e.Content,
						ModelName: a.preset.Name,
						Provider:  a.preset.Provider,
					}

					// Save tool calls
					toolCalls = e.ToolCalls
					if len(toolCalls) > 0 {
						toolCallsString, err := json.Marshal(toolCalls)
						if err != nil {
							eventsChan <- &events.ErrorEvent{
								Error: fmt.Errorf("Failed to parse ToolCalls: %w", err),
							}
							return
						}
						aiMsg.ToolCalls = string(toolCallsString)
					}

					if err := a.repository.AddMessageToThread(ctx, opts.ThreadID, aiMsg); err != nil {
						eventsChan <- &events.ErrorEvent{
							Error: fmt.Errorf("failed to add AI message to thread: %w", err),
						}
						return
					}

					// Send AI message event
					eventsChan <- &NewMessageEvent{
						Message: aiMsg,
					}

					// Check for function calls requiring approval
					if len(toolCalls) > 0 {
						var toolsNeedingApproval []llm.ToolCall

						for _, call := range toolCalls {
							// Find tool approval setting
							for serverName, serverTools := range a.tools {
								for toolName, tool := range serverTools {
									if fmt.Sprintf("%s__%s", serverName, toolName) == call.Name {
										if tool.RequireApproval {
											toolsNeedingApproval = append(toolsNeedingApproval, call)
										}
									}
								}
							}
						}

						// If any tools need approval, emit an approval event
						if len(toolsNeedingApproval) > 0 {
							eventsChan <- &ToolApprovalEvent{
								Message:   aiMsg,
								ToolCalls: toolsNeedingApproval,
							}
							return
						}

						// All tools are auto-approved, execute them
						results, err := a.ExecuteTools(ctx, toolCalls)
						if err != nil {
							if ctx.Err() != nil {
								// Prioritize reporting context errors
								eventsChan <- &events.ErrorEvent{Error: ctx.Err()}
							} else {
								eventsChan <- &events.ErrorEvent{Error: fmt.Errorf("failed to execute tools: %w", err)}
							}
							return
						}

						// Create tool result message
						toolMsg := &domain.Message{
							ThreadID: opts.ThreadID,
							ParentID: &aiMsg.ID,
							Role:     domain.RoleTool,
							Content:  results,
						}

						if err := a.repository.AddMessageToThread(ctx, opts.ThreadID, toolMsg); err != nil {
							eventsChan <- &events.ErrorEvent{
								Error: fmt.Errorf("failed to add tool results to thread: %w", err),
							}
							return
						}

						// Send tool result event
						eventsChan <- &NewMessageEvent{
							Message: toolMsg,
						}
					}
				case *events.ErrorEvent:
					eventsChan <- e
					return
				}

			case <-llmStream.Done:
				return
			}
		}
	}()

	return AgentStream{Events: eventsChan, Done: done}
}
