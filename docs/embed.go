// Package docs содержит встроенную HTML-документацию проекта.
package docs

import _ "embed"

// HTML содержит страницу документации docs.html.
//
//go:embed docs.html
var HTML []byte
