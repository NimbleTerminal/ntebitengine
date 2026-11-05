# Changelog

## [Unreleased]

## [0.1.2] - 2026-11-05

### Changed

- Migrate browser demos and the asset tool from `github.com/NimbleMarkets/go-booba` to `nimbleterminal.dev/booba` v0.12.1.
- Update ntpicture to v0.1.5, ntcanvas to v0.1.4, purego to v0.11.3, and terminfo to v1.3.1.
- Check the generated `ntwebterm` runtime assets in the site build and exclude them from version control.

## [0.1.1] - 2026-10-30

### Changed

- Isolate examples and browser tooling in a separate Go module, with matching CI, build commands, and documentation.
- Update ntpicture to v0.1.4 and refresh Go dependencies.

## [0.1.0] - 2026-10-21

### Added

- Initial release of `nimbleterminal.dev/ntebitengine` for rendering Ebitengine framebuffers inside Bubble Tea applications through ntpicture.
- A hidden Ebitengine loop with a portable main-thread `Run` path, reusable GPU surfaces, and immutable premultiplied RGBA readback.
- Presentation-paced rendering with one request in flight, cancellation, and stale-frame rejection on geometry changes.
- Diagnostics for drawing, readback, presentation, and frame discards.
- Six native and browser demos covering vector bounce, sprites, text, Kage shaders, event-loop coordination, and ntui overlays.
- Guides for event-loop ownership, state snapshots, shutdown, and frame-cost measurement.
- CI for vet, race detection, graphics integration tests, and GitHub Pages deployment.

[Unreleased]: https://github.com/NimbleTerminal/ntebitengine/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/NimbleTerminal/ntebitengine/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/NimbleTerminal/ntebitengine/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/NimbleTerminal/ntebitengine/releases/tag/v0.1.0
