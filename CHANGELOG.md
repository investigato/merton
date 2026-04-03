
# Change Log

Here's what investigato has been up to:

## [0.0.4] - 2026-04-03 - `yes-i-pushed-to-prod-on-friday`

### Changed

- Increased number of retries to 5 on connections.
- Decreased chunk size in downloads to 64KB

### Fixed

- `type` & `cat` now show output correctly instead of a single-line in a multi-line file.
- Downloads after Kerberos auth are pretty stable. Downloads after NTLM auth are...better than they were.

## [0.0.3] - 2026-04-03 - `it-works-on-my-machine`

### Added

- `cat` now maps to `type` because we forget we're not using Linux
- `github.com/fatih/color` added to dependencies

### Fixed

- `type` & `cat` were showing errors instead of answers. This has been corrected.

## [0.0.2] - 2026-04-03 - `let-me-try-again`

### Fixed

- Encryption for NTLM auth now defaults to `true`

## [0.0.1] - 2026-04-02 - `percent-of-the-time-it-works-everytime`

Initial release
