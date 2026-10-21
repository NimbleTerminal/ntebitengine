package main

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func TestWrapPreservesTextAndFitsMeasuredWidth(t *testing.T) {
	source, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		t.Fatal(err)
	}
	face := &text.GoTextFace{Source: source, Size: 20}
	for _, value := range []string{
		"Good type gives a scene its voice.",
		"supercalifragilisticexpialidocious",
		"Приветмирκόσμε",
		"",
	} {
		for _, width := range []float64{40, 120, 600} {
			lines := wrap(value, face, width)
			if got, want := strings.ReplaceAll(strings.Join(lines, ""), " ", ""), strings.ReplaceAll(value, " ", ""); got != want {
				t.Errorf("width %g: text changed: %q != %q", width, got, want)
			}
			for _, line := range lines {
				if !utf8.ValidString(line) || line == "" {
					t.Errorf("invalid line %q", line)
				}
				if w, _ := text.Measure(line, face, 0); w > width {
					t.Errorf("%q is %g wide, exceeds %g", line, w, width)
				}
			}
		}
	}
	const paragraph = "Good type gives a scene its voice. Resize to reflow."
	if len(wrap(paragraph, face, 100)) <= len(wrap(paragraph, face, 300)) {
		t.Error("a narrower paragraph should require more lines")
	}
}
