// Package task defines the Task — the atomic routable unit of work in Fort.
// A Task carries only deterministic signals; routing never inspects model output.
package task

import "time"

// Task is a unit of work sourced from the inbox (CLI add, watched file, or a
// label feed) and routed to exactly one agent by the deterministic router.
type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body,omitempty"`
	Labels    []string  `json:"labels,omitempty"`
	Paths     []string  `json:"paths,omitempty"` // files the task touches
	Repo      string    `json:"repo,omitempty"`
	Agent     string    `json:"agent,omitempty"` // explicit @agent override
	Size      string    `json:"size,omitempty"`  // S | M | L | XL
	CreatedAt time.Time `json:"created_at"`
}
