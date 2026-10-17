package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Answer int

const (
	AnswerNo Answer = iota
	AnswerYes
	AnswerList    // show every file, then ask again
	AnswerNoInput // the input ended before an answer
)

// Prompt asks questions; it keeps one reader so that repeated questions do
// not lose buffered input.
type Prompt struct {
	in   *bufio.Reader
	out  io.Writer
	echo bool // the input is not a terminal, so the answer is not seen in the output
}

// NewPrompt reads answers from in. With echo the answer is followed by a new
// line, as a terminal would show it.
func NewPrompt(in io.Reader, out io.Writer, echo bool) *Prompt {
	return &Prompt{in: bufio.NewReader(in), out: out, echo: echo}
}

// Ask shows "question [y/N]" or "question [y/N/l]". Only "y" and "yes" in any
// case mean yes, "l" and "list" ask for the list; anything else means no.
func (p *Prompt) Ask(question string, allowList bool) Answer {
	choices := "[y/N]"
	if allowList {
		choices = "[y/N/l]"
	}
	fmt.Fprintf(p.out, "%s %s ", question, choices)

	line, err := p.in.ReadString('\n')
	if p.echo || err != nil {
		fmt.Fprintln(p.out)
	}
	if err != nil && line == "" {
		return AnswerNoInput
	}

	switch answer := strings.ToLower(strings.TrimSpace(line)); {
	case answer == "y" || answer == "yes":
		return AnswerYes
	case allowList && (answer == "l" || answer == "list"):
		return AnswerList
	default:
		return AnswerNo
	}
}
