# Text lab

```sh
go run ./examples/text
```

Three scenes exercise Ebitengine's `text/v2` API through the same live-framebuffer
producer as the other examples:

- **Specimen:** a headline, character samples, and three font cards.
- **Paragraph:** measured word wrapping and start/center/end alignment.
- **Motion:** cached glyphs with rotation, scaling, and translucent trails.

| Key | Action |
| --- | --- |
| `t` | Next scene |
| `f` | Next font: Go Sans, Go Bold, Go Mono |
| `a` | Start, center, or end alignment (specimen and paragraph) |
| `n` | Next headline, including Greek and Cyrillic samples |
| `[` / `]` | Smaller / larger type |
| Space | Pause / resume animation |
| `m` | Toggle Kitty graphics / half-block glyphs |
| `+` / `-` | Increase / decrease Kitty raster resolution |
| `q`, Esc, Ctrl+C | Quit |

Resize the terminal to reflow the paragraph. Text is measured with the selected
font, with oversized words split at rune boundaries and an ellipsis when the
paragraph exceeds the available height. This simple wrapper is not a complete
Unicode line-breaking implementation. Headline sizes are capped to fit their
reserved area.

Font sources are initialized once on the drawing goroutine. Font sizes follow
the raster's vertical pixel scale, and horizontal transforms compensate for
non-square terminal pixels. Animated scaling uses a transform so it can reuse
the glyph cache. Control changes cross from Bubble Tea through immutable atomic
snapshots; font caches and animation state stay on the drawing goroutine.

The bundled fonts come from `golang.org/x/image/font/gofont` under the
[Go font license](https://go.googlesource.com/image/+/refs/heads/master/font/gofont/ttfs/LICENSE).
No system font installation or runtime download is needed. These fonts do not
cover every script or emoji. Kitty graphics at higher resolution are recommended
for reading small text; the half-block framebuffer has much less detail.

FPS counts the picture's committed `PresentedMsg` frames, not Ebitengine's loop
ticks or terminal acknowledgements. Pausing freezes animation while presentation
continues so font, layout, and resolution controls remain live.

The browser page runs this same program at `web/?demo=text` after
`sh scripts/build-site.sh`. See the repository README for serving instructions.
