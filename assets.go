package shiroyagiassets

import "embed"

// Files contains the server-rendered templates and browser assets required by
// the web application.
//
//go:embed web/templates/*.html web/templates/partials/*.html static/css/app.css static/js/htmx.min.js static/js/LICENSE.htmx.txt
var Files embed.FS
