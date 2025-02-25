package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/isaacphi/slop/internal/domain"
	"github.com/isaacphi/slop/internal/events"
	"github.com/isaacphi/slop/internal/llm"
)

// SendMessageStream sends a message through the Agent and returns a stream of events
// It takes a domain.Message as input and handles both new messages and tool approvals
func (a *Agent) SendMessageStream(ctx context.Context, msg *domain.Message) AgentStream {
	eventsChan := make(chan events.Event)
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer close(eventsChan)

		// Start the agent loop
		err := a.agentLoop(ctx, msg, eventsChan)
		if err != nil {
			eventsChan <- &events.ErrorEvent{
				Error: err,
			}
		}
	}()

	return AgentStream{Events: eventsChan, Done: done}
}

// agentLoop handles the continuous processing of messages and tool calls
func (a *Agent) agentLoop(ctx context.Context, initialMsg *domain.Message, eventsChan chan events.Event) error {
	// Validate thread exists
	thread, err := a.repository.GetThread(ctx, initialMsg.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}

	// Use iteration instead of recursion to avoid stack overflow
	currentMsg := initialMsg

	for {
		// Check context cancellation at the start of each iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		// Different handling based on message role
		switch currentMsg.Role {
		case domain.RoleAssistant:
			// Handle existing assistant message with tool calls (approval flow)
			if currentMsg.ToolCalls == "" {
				return fmt.Errorf("assistant message has no tool calls")
			}

			// Parse tool calls from the message
			var toolCalls []llm.ToolCall
			if err := json.Unmarshal([]byte(currentMsg.ToolCalls), &toolCalls); err != nil {
				return fmt.Errorf("failed to parse tool calls: %w", err)
			}

			if len(toolCalls) == 0 {
				return fmt.Errorf("no tool calls found in message")
			}

			// Execute the approved tools and continue the loop
			results, err := a.ExecuteTools(ctx, toolCalls)
			if err != nil {
				return fmt.Errorf("failed to execute tools: %w", err)
			}

			// Send tool execution events
			for _, call := range toolCalls {
				eventsChan <- &ToolResultEvent{
					ToolCallID: call.ID,
					Name:       call.Name,
					Result:     results,
				}
			}

			// Create tool result message
			toolMsg := &domain.Message{
				ThreadID: currentMsg.ThreadID,
				ParentID: &currentMsg.ID,
				Role:     domain.RoleTool,
				Content:  results,
			}

			if err := a.repository.AddMessageToThread(ctx, currentMsg.ThreadID, toolMsg); err != nil {
				return fmt.Errorf("failed to add tool results to thread: %w", err)
			}

			// Send message created event
			eventsChan <- &NewMessageEvent{
				Message: toolMsg,
			}

			// Continue the loop with the tool message
			currentMsg = toolMsg

		case domain.RoleHuman, domain.RoleTool:
			// Store message if it doesn't have an ID yet (new message)
			if currentMsg.ID == uuid.Nil {
				if err := a.repository.AddMessageToThread(ctx, thread.ID, currentMsg); err != nil {
					return fmt.Errorf("failed to add message to thread: %w", err)
				}

				// Send message created event
				eventsChan <- &NewMessageEvent{
					Message: currentMsg,
				}
			}

			// Get the AI response
			aiMsg, shouldContinue, err := a.processMessage(ctx, currentMsg, eventsChan)
			if err != nil {
				return err
			}

			// If we shouldn't continue, exit the loop
			if !shouldContinue {
				return nil
			}

			// Update current message to continue the loop
			currentMsg = aiMsg

		default:
			return fmt.Errorf("unsupported message role: %s", currentMsg.Role)
		}
	}
}

// processMessage generates the next AI response based on the given message
// Returns the AI message, a boolean indicating if the loop should continue, and any error
func (a *Agent) processMessage(ctx context.Context, msg *domain.Message, eventsChan chan events.Event) (*domain.Message, bool, error) {
	// Get conversation history for context
	history, err := a.repository.GetMessages(ctx, msg.ThreadID, msg.ParentID, false)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get conversation history: %w", err)
	}

	// Build system message
	systemMessage, err := a.buildSystemMessage(systemMessageOpts{
		messageContent: msg.Content,
		history:        history,
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to build system message: %w", err)
	}

	// Get AI response
	generateOptions := llm.GenerateContentOptions{
		Preset:        a.preset,
		Content:       msg.Content,
		SystemMessage: systemMessage,
		History:       history,
		Tools:         flattenTools(a.tools),
	}

	// Get LLM stream
	llmStream := llm.GenerateContentStream(ctx, generateOptions)

	// Track assistant response for saving
	var aiMsg *domain.Message
	var toolCalls []llm.ToolCall

	// Forward LLM events to agent stream
	for {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()

		case event, ok := <-llmStream.Events:
			if !ok {
				if aiMsg == nil {
					// Stream closed without completing the message
					return nil, false, nil
				}
				// If we have no tool calls, we're done with the loop
				return aiMsg, false, nil
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
				aiMsg = &domain.Message{
					ThreadID:  msg.ThreadID,
					ParentID:  &msg.ID,
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
						return nil, false, fmt.Errorf("failed to parse ToolCalls: %w", err)
					}
					aiMsg.ToolCalls = string(toolCallsString)
				}

				if err := a.repository.AddMessageToThread(ctx, msg.ThreadID, aiMsg); err != nil {
					return nil, false, fmt.Errorf("failed to add AI message to thread: %w", err)
				}

				// Send AI message event
				eventsChan <- &NewMessageEvent{
					Message: aiMsg,
				}

				// If no tool calls, we're done with the loop
				if len(toolCalls) == 0 {
					return aiMsg, false, nil
				}

				// Check for function calls requiring approval
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

				// If any tools need approval, emit an approval event and exit the loop
				if len(toolsNeedingApproval) > 0 {
					eventsChan <- &ToolApprovalRequestEvent{
						Message:   aiMsg,
						ToolCalls: toolsNeedingApproval,
					}
					return aiMsg, false, nil
				}

				// All tools are auto-approved, execute them
				results, err := a.ExecuteTools(ctx, toolCalls)
				if err != nil {
					if ctx.Err() != nil {
						// Prioritize reporting context errors
						return nil, false, ctx.Err()
					} else {
						return nil, false, fmt.Errorf("failed to execute tools: %w", err)
					}
				}

				// Send tool execution events
				for _, call := range toolCalls {
					eventsChan <- &ToolResultEvent{
						ToolCallID: call.ID,
						Name:       call.Name,
						Result:     results,
					}
				}

				// Create tool result message
				toolMsg := &domain.Message{
					ThreadID: msg.ThreadID,
					ParentID: &aiMsg.ID,
					Role:     domain.RoleTool,
					Content:  results,
				}

				if err := a.repository.AddMessageToThread(ctx, msg.ThreadID, toolMsg); err != nil {
					return nil, false, fmt.Errorf("failed to add tool results to thread: %w", err)
				}

				// Send message created event
				eventsChan <- &NewMessageEvent{
					Message: toolMsg,
				}

				// Continue the agent loop with the tool message
				return toolMsg, true, nil

			case *events.ErrorEvent:
				return nil, false, e.Error
			}

		case <-llmStream.Done:
			if aiMsg == nil {
				// Stream closed without completing the message
				return nil, false, nil
			}
			// If we have no tool calls, we're done with the loop
			return aiMsg, false, nil
		}
	}
}
