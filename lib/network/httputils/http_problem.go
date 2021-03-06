package httputils

import (
	"boscoin.io/sebak/lib/error"
	"fmt"
	"net/http"
)

const (
	HttpProblemDefaultType     = "about:blank" // It should be URI
	HttpProblemErrorTypePrefix = "https://boscoin.io/sebak/error/"
)

type problem struct {
	// "type" (string) - A URI reference [RFC3986] that identifies the
	// problem type.  This specification encourages that, when
	// dereferenced, it provide human-readable documentation for the
	// problem type (e.g., using HTML [W3C.REC-html5-20141028]).  When
	// this member is not present, its value is assumed to be
	// "about:blank".
	Type string `json:"type"`

	//"title" (string) - A short, human-readable summary of the problem
	//type.  It SHOULD NOT change from occurrence to occurrence of the
	//problem, except for purposes of localization (e.g., using
	//proactive content negotiation; see [RFC7231], Section 3.4).
	Title string `json:"title"`

	//"status" (number) - The HTTP status code ([RFC7231], Section 6)
	//generated by the origin server for this occurrence of the problem.
	Status int `json:"status,omitempty"`

	//"detail" (string) - A human-readable explanation specific to this
	//occurrence of the problem.
	Detail string `json:"detail,omitempty"`

	//"instance" (string) - A URI reference that identifies the specific
	//occurrence of the problem.  It may or may not yield further
	//information if dereferenced.
	Instance string `json:"instance,omitempty"`
}

func NewProblem(problemType string, title string) problem {
	return problem{Type: problemType, Title: title}
}

func NewStatusProblem(status int) problem {
	p := NewProblem(HttpProblemDefaultType, http.StatusText(status))
	p.Status = status
	return p
}

func NewDetailedStatusProblem(status int, detail string) problem {
	p := NewStatusProblem(status)
	p.Detail = detail
	return p
}

func NewErrorProblem(err error, status int) problem {
	var p problem
	if e, ok := err.(*errors.Error); ok {
		p = NewProblem(fmt.Sprintf("%s%d", HttpProblemErrorTypePrefix, e.Code), e.Message)
	} else {
		p = NewProblem(HttpProblemDefaultType, err.Error())
	}
	p.Status = status
	return p
}

func (p problem) SetInstance(instance string) problem {
	p.Instance = instance
	return p
}

func (p problem) SetStatus(status int) problem {
	p.Status = status
	return p
}

func (p problem) SetDetail(detail string) problem {
	p.Detail = detail
	return p
}
