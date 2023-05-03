package action

import (
	"fmt"
	"log"
	model "pkdindustries/soulshack/model"
	"regexp"
	"strings"
)

var regex = regexp.MustCompile(`(^|\n)Action:`)

type ReactAction interface {
	Name() string
	Purpose() string
	Execute(model.ChatContext, string) (string, error)
	Spec() string
}

type ReactConfig struct {
	Actions map[string]ReactAction
}

func ReactPrompt(config *ReactConfig) string {
	var tools []string
	for _, action := range config.Actions {
		tools = append(tools, fmt.Sprintf("%s:%s", action.Name(), action.Purpose()))
		tools = append(tools, fmt.Sprintf("example: %s", action.Spec()))
	}

	template := fmt.Sprintf(`you have access to the following tools:
%s
Use the following format:
Question: the input question you must answer
Thought: you should always think about what to do
Action: the action to take, should be one of [{$toolname}]

** you will issue a single Action at a time, and you will then STOP and wait for an Observation reply **

Observation: the result of the action
... (this Thought/Action/Observation can repeat N times)
Thought: I now know the final answer
Final Answer: the final answer to the original input question

Question:`, strings.Join(tools, "\n"))

	return template
}

func ReactActionObservation(ctx model.ChatContext, msg string) {
	if action := ReactFindActions(msg); action != "" {
		log.Println("action found:", action)
		r, e := React(ctx, &ReactConfig{Actions: ActionRegistry}, strings.TrimSpace(action))
		if e != nil {
			log.Println(e)
			ctx.Complete("Observation: " + e.Error())
		} else {
			log.Println(r)
			ctx.Complete("Observation: " + r)
		}
	}
}

func ReactFindActions(msg string) string {
	if loc := regex.FindStringIndex(msg); loc != nil {
		action := msg[loc[0]:]
		return action
	}
	return ""
}

func React(ctx model.ChatContext, cfg *ReactConfig, msg string) (string, error) {
	parts := strings.SplitN(msg, ": ", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid input: %s", msg)
	}
	args := strings.Split(parts[1], " ")
	action := strings.TrimSpace(args[0])

	ra, ok := cfg.Actions[action]
	if !ok {
		log.Println("Action not found:", action)
		return "", fmt.Errorf("action not found: %s", action)
	}

	return ra.Execute(ctx, parts[1])
}

func ReactFilter(ctx model.ChatContext, cfg *ReactConfig, in <-chan string) <-chan string {
	outch := make(chan string)
	go func() {
		defer close(outch)
		for msg := range in {
			if msg == "" {
				return
			}
			out, err := React(ctx, cfg, msg)
			if err != nil {
				outch <- fmt.Sprintf("Error: %s", err)
				return
			}
			outch <- "Observation: " + out
		}
	}()
	return outch
}

// func Reactor(cfg *ReactConfig, in chan *string) <-chan *ReactEvent {
// 	outch := make(chan *ReactEvent)
// 	go func() {
// 		defer close(outch)
// 		for msg := range in {
// 			if msg == nil {
// 				return
// 			}

// 			//	FindActions(*msg)
// 			parts := strings.SplitN(*msg, ": ", 2)
// 			if len(parts) != 2 {
// 				outch <- &ReactEvent{Type: Error, Data: fmt.Sprintf("Invalid input: %s", *msg)}
// 				continue
// 			}

// 			event := strings.TrimSpace(parts[0])
// 			args := strings.Split(parts[1], " ")
// 			action := strings.TrimSpace(args[0])

// 			switch event {
// 			case "Thought":
// 				outch <- &ReactEvent{Type: Thought, Data: parts[1]}
// 			case "Action":
// 				_, ok := cfg.Actions[action]
// 				if ok {
// 					outch <- &ReactEvent{Type: Action, Data: parts[1]}
// 				} else {
// 					outch <- &ReactEvent{Type: Error, Data: fmt.Sprintf("Unknown Action: %s", action)}
// 				}
// 			case "Observation":
// 				outch <- &ReactEvent{Type: Observation, Data: parts[1]}
// 			case "Final Thought":
// 				outch <- &ReactEvent{Type: FinalThought, Data: parts[1]}
// 			case "Final Answer":
// 				outch <- &ReactEvent{Type: FinalAnswer, Data: parts[1]}
// 			default:
// 				log.Printf("Unknown event type: %s", event)
// 			}
// 		}
// 	}()
// 	return outch
// }
