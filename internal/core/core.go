package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sammwy/teragen/internal/agent"
	"github.com/sammwy/teragen/internal/processor"
	"github.com/sammwy/teragen/internal/workspace"
)

type EventType string

const (
	EventChatVisualClear   EventType = "chat.visual_clear"
	EventChatInternalClear EventType = "chat.internal_clear"
	EventChatSwitched      EventType = "chat.switched"
	EventStreamChunk       EventType = "stream.chunk"
	EventStreamDone        EventType = "stream.done"
	EventError             EventType = "error"
	EventCommandResult     EventType = "command.result"
	EventAgentAdded        EventType = "agent.added"
	EventAgentRemoved      EventType = "agent.removed"
)

type Event struct {
	Type    EventType
	Data    interface{}
	AgentID string
}

type Subscriber interface {
	OnEvent(Event)
}

type Core struct {
	Agents      map[string]*agent.Agent
	AgentIDs    []string
	Subscribers []Subscriber
	Processor   *processor.Processor
	Workspace   workspace.Workspace
	mu          sync.RWMutex
}

func NewCore(p *processor.Processor, ws workspace.Workspace) *Core {
	return &Core{
		Agents:    make(map[string]*agent.Agent),
		Processor: p,
		Workspace: ws,
	}
}

func (c *Core) RegisterAgent(id string, a *agent.Agent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.Agents[id]; !ok {
		c.AgentIDs = append(c.AgentIDs, id)
	}
	c.Agents[id] = a
}

func (c *Core) UnregisterAgent(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Agents, id)
	for i, aid := range c.AgentIDs {
		if aid == id {
			c.AgentIDs = append(c.AgentIDs[:i], c.AgentIDs[i+1:]...)
			break
		}
	}
}

func (c *Core) GetAgentList() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]string(nil), c.AgentIDs...)
}

func (c *Core) CreateAgent() (string, *agent.Agent, error) {
	c.mu.Lock()
	max := 0
	for _, id := range c.AgentIDs {
		var n int
		fmt.Sscanf(id, "%d", &n)
		if n > max {
			max = n
		}
	}
	newID := fmt.Sprintf("%d", max+1)
	c.mu.Unlock()

	ag, err := agent.NewAgent(c.Workspace, workspace.AgentSession{
		ID: newID,
	})
	if err != nil {
		return "", nil, err
	}

	c.RegisterAgent(newID, ag)
	return newID, ag, nil
}

func (c *Core) Subscribe(s Subscriber) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Subscribers = append(c.Subscribers, s)
}

func (c *Core) Emit(e Event) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, s := range c.Subscribers {
		s.OnEvent(e)
	}
}

func (c *Core) DispatchPrompt(ctx context.Context, agentID string, input string) {
	a, ok := c.Agents[agentID]
	if !ok {
		c.Emit(Event{Type: EventError, Data: fmt.Errorf("agent %s not found", agentID), AgentID: agentID})
		return
	}

	go func() {
		// Handle Commands
		if strings.HasPrefix(input, "/") {
			resp, _, err := c.Processor.Process(ctx, a, input)
			if err != nil {
				c.Emit(Event{Type: EventError, Data: err, AgentID: agentID})
				return
			}

			if resp == "__CLEAR_VISUAL__" {
				c.Emit(Event{Type: EventChatVisualClear, AgentID: agentID})
			} else if resp == "__CLEAR_INTERNAL__" {
				c.Emit(Event{Type: EventChatInternalClear, AgentID: agentID})
			} else if strings.HasPrefix(resp, "__CHAT_SWITCHED__") {
				newID := strings.TrimPrefix(resp, "__CHAT_SWITCHED__")
				c.Emit(Event{Type: EventChatSwitched, Data: newID, AgentID: agentID})
			} else {
				c.Emit(Event{Type: EventCommandResult, Data: resp, AgentID: agentID})
			}
			return
		}

		// Handle Chat Stream
		ch, err := a.Stream(ctx, input)
		if err != nil {
			c.Emit(Event{Type: EventError, Data: err, AgentID: agentID})
			return
		}

		for ev := range ch {
			if ev.Err != nil {
				c.Emit(Event{Type: EventError, Data: ev.Err, AgentID: agentID})
				return
			}

			if ev.Done {
				c.Emit(Event{Type: EventStreamDone, Data: ev, AgentID: agentID})
			} else {
				c.Emit(Event{Type: EventStreamChunk, Data: ev, AgentID: agentID})
			}
		}
	}()
}
