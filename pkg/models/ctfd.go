package models

import (
	"encoding/json"
	"time"
)

type TagList []string

func (t *TagList) UnmarshalJSON(data []byte) error {
	var stringArray []string
	if err := json.Unmarshal(data, &stringArray); err == nil {
		*t = TagList(stringArray)
		return nil
	}

	var tagObjects []interface{}
	if err := json.Unmarshal(data, &tagObjects); err == nil {
		tags := make([]string, 0, len(tagObjects))
		for _, obj := range tagObjects {
			switch v := obj.(type) {
			case string:
				tags = append(tags, v)
			case map[string]interface{}:
				if value, ok := v["value"].(string); ok {
					tags = append(tags, value)
				} else if name, ok := v["name"].(string); ok {
					tags = append(tags, name)
				}
			}
		}
		*t = TagList(tags)
		return nil
	}

	var emptyObj interface{}
	if err := json.Unmarshal(data, &emptyObj); err == nil {
		*t = TagList([]string{})
		return nil
	}

	*t = TagList([]string{})
	return nil
}

type ChallengeListResponse struct {
	Success bool        `json:"success"`
	Data    []Challenge `json:"data"`
}

type ChallengeDetailResponse struct {
	Success bool              `json:"success"`
	Data    ChallengeDetailed `json:"data"`
}

type Challenge struct {
	ID         int     `json:"id"`
	Type       string  `json:"type"`
	Name       string  `json:"name"`
	Value      int     `json:"value"`
	Position   int     `json:"position"`
	Solves     *int    `json:"solves"`
	SolvedByMe bool    `json:"solved_by_me"`
	Category   string  `json:"category"`
	Tags       TagList `json:"tags"`
	Template   string  `json:"template"`
	Script     string  `json:"script"`
}

type ChallengeDetailed struct {
	Challenge
	Description    string         `json:"description"`
	Attribution    *string        `json:"attribution"`
	ConnectionInfo *string        `json:"connection_info"`
	NextID         *int           `json:"next_id"`
	MaxAttempts    int            `json:"max_attempts"`
	State          string         `json:"state"`
	Logic          string         `json:"logic"`
	Initial        *int           `json:"initial"`
	Minimum        *int           `json:"minimum"`
	Decay          *int           `json:"decay"`
	Function       string         `json:"function"`
	Requirements   interface{}    `json:"requirements"`
	Attempts       int            `json:"attempts"`
	Files          []string       `json:"files"`
	Hints          []Hint         `json:"hints"`
	Rating         *Rating        `json:"rating"`
	Ratings        *RatingSummary `json:"ratings"`
	SolutionID     *int           `json:"solution_id"`
	SolutionState  string         `json:"solution_state"`
	View           string         `json:"view"`
}

type Hint struct {
	ID      int     `json:"id"`
	Cost    int     `json:"cost"`
	Title   string  `json:"title"`
	Content *string `json:"content"`
}

type Rating struct {
	Value  int    `json:"value"`
	Review string `json:"review"`
}

type RatingSummary struct {
	Up    int `json:"up"`
	Down  int `json:"down"`
	Count int `json:"count"`
}

type AttemptRequest struct {
	ChallengeID int    `json:"challenge_id"`
	Submission  string `json:"submission"`
}

type AttemptResponse struct {
	Success bool        `json:"success"`
	Data    AttemptData `json:"data"`
}

type AttemptData struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SolvesResponse struct {
	Success bool    `json:"success"`
	Data    []Solve `json:"data"`
}

type Solve struct {
	AccountID int       `json:"account_id"`
	Name      string    `json:"name"`
	Date      time.Time `json:"date"`
	Account   string    `json:"account"`
}

type FileToken struct {
	UserID int `json:"user_id"`
	TeamID int `json:"team_id,omitempty"`
	FileID int `json:"file_id"`
}

type ChallengeFile struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	Location string `json:"location"`
	SHA1Sum  string `json:"sha1sum"`
}

type ErrorResponse struct {
	Success bool                `json:"success"`
	Errors  map[string][]string `json:"errors"`
}

type SimpleErrorResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors"`
}

type PaginatedResponse struct {
	Success bool           `json:"success"`
	Data    interface{}    `json:"data"`
	Meta    PaginationMeta `json:"meta"`
}

type PaginationMeta struct {
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page    int `json:"page"`
	Next    int `json:"next"`
	Prev    int `json:"prev"`
	Pages   int `json:"pages"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}
