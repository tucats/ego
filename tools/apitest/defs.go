package main

import "time"

type RequestObject struct {
	Endpoint   string              `json:"endpoint"`
	Parameters map[string]string   `json:"parameters,omitempty"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Method     string              `json:"method"`
	Body       string              `json:"body,omitempty"`
}

type ResponseObject struct {
	Headers map[string][]string `json:"headers"`
	Status  int                 `json:"status"`
	Body    string              `json:"body"`
	Save    map[string]string   `json:"save,omitempty"`
}

type Validation struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Value      string `json:"value,omitempty"`
	Operator   string `json:"operator"`
}

type Test struct {
	Description string         `json:"description"`
	Request     RequestObject  `json:"request"`
	Response    ResponseObject `json:"response"`
	Tests       []Validation   `json:"tests,omitempty"`
	Succeeded   bool           `json:"success"`
	Time        time.Time      `json:"time,omitempty"`
}
