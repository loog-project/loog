package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/davecgh/go-spew/spew"
)

const input = `---
plugins:
  loog-run-selected:
    shortCut: Shift-L
    confirm: false
    scopes:
      - all
    description: "👀: Run Selected"
    command: bash
    background: false
    args:
      - -c
      - >
        loogtui
        -resource
        $RESOURCE_GROUP/$RESOURCE_VERSION/$RESOURCE_NAME
        -filter-expr
        'Namespaced("$NAMESPACE", "$NAME")'
  loog-run-namespace:
    shortCut: Ctrl-L
    confirm: true
    scopes:
      - all
    description: "👀: Run Namespace"
    command: bash
    background: false
    args:
      - -c
      - >
        loogtui
        -resource
        $RESOURCE_GROUP/$RESOURCE_VERSION/$RESOURCE_NAME
        -filter-expr
        'Namespace("$NAMESPACE")'
`

func main() {
	isDark := lipgloss.HasDarkBackground()

	lexer := lexers.Get("yaml")
	if lexer == nil {
		panic("unable to get YAML lexer")
	}

	var styleChoice string
	if isDark {
		styleChoice = "dracula"
	} else {
		styleChoice = "github"
	}

	style := styles.Get(styleChoice)
	if style == nil {
		panic("unable to get style: " + styleChoice)
	}
	formatter := formatters.TTY16m

	iterator, err := lexer.Tokenise(nil, input)
	if err != nil {
		panic(err)
	}

	for token := range iterator.Stdlib() {
		spew.Dump(token)
	}

	if err := formatter.Format(os.Stdout, style, iterator); err != nil {
		panic(err)
	}

	dec := json.NewDecoder(bytes.NewReader([]byte(`{
	"plugins": {
		"loog-run-selected": {
			"shortCut": "Shift-L",
			"confirm": false,
			"scopes": ["all"],
			"description": "👀: Run Selected",
			"command": "bash",
			"background": false,
			"args": [
				"-c",
				"loogtui -resource $RESOURCE_GROUP/$RESOURCE_VERSION/$RESOURCE_NAME -filter-expr 'Namespaced(\"$NAMESPACE\", \"$NAME\")'"
			]
		}
	}`)))

	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		spew.Dump(token)
	}
}
