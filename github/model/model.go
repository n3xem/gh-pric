package model

import (
	"time"
)

// Struct for setting the date range
type DateRange struct {
	StartDate time.Time
	EndDate   time.Time
}

// Struct to hold information about PRs and Issues
type Item struct {
	Type        string    // "PR" or "Issue"
	Number      int       // PR number or Issue number
	Title       string    // Title
	URL         string    // URL
	State       string    // State (open, closed, merged)
	CreatedAt   time.Time // Creation date
	UpdatedAt   time.Time // Update date
	Author      string    // Author
	Assignees   []string  // Assignees
	Labels      []string  // Labels
	Repository  string    // Repository name
	Involvement string    // Involvement type (created, assigned, commented)
	Body        string    // Body
	Comments    []Comment // Comments
}

// Struct to hold comment information
type Comment struct {
	Author    string    // Comment author
	Body      string    // Comment body
	CreatedAt time.Time // Date of posting
	UpdatedAt time.Time // Update date
} 
