package render

import (
	"html"
	"html/template"
	"strings"
	"unicode/utf8"
)

type quotation struct {
	Full, Preview template.HTML
	Long          bool
}

func quoteView(source string) quotation {
	q := quotation{Full: formatQuote(source, utf8.RuneCountInString(source))}
	q.Long = utf8.RuneCountInString(source) > 200
	if q.Long {
		q.Preview = formatQuote(source, 200) + "…"
	}
	return q
}

// Parse the complete source before limiting its visible characters. A closing
// delimiter beyond the preview still closes the generated HTML element.
func formatQuote(source string, limit int) template.HTML {
	var out strings.Builder
	var render func(string, int, bool)
	render = func(s string, remaining int, bold bool) {
		literal := func(text string) {
			runes := []rune(text)
			if len(runes) > remaining {
				runes = runes[:remaining]
			}
			out.WriteString(html.EscapeString(string(runes)))
			remaining -= len(runes)
		}
		for len(s) > 0 && remaining > 0 {
			delimiter, tag := "", ""
			switch {
			case strings.HasPrefix(s, "`"):
				delimiter, tag = "`", "code"
			case !bold && strings.HasPrefix(s, "**"):
				delimiter, tag = "**", "strong"
			case !bold && strings.HasPrefix(s, "__"):
				delimiter, tag = "__", "strong"
			}
			if delimiter != "" {
				// Runs longer than the supported delimiters stay literal.
				n := len(delimiter)
				for n < len(s) && s[n] == s[0] {
					n++
				}
				if n > len(delimiter) {
					literal(s[:n])
					s = s[n:]
					continue
				}
				end := quoteDelimiter(s, delimiter)
				if end > len(delimiter) {
					content := s[len(delimiter):end]
					visible := remaining - len(delimiter)
					if visible > 0 {
						out.WriteString("<" + tag + ">")
						if tag == "code" {
							remaining = visible
							literal(content)
						} else {
							render(content, visible, true)
						}
						out.WriteString("</" + tag + ">")
					}
					remaining = visible - utf8.RuneCountInString(content) - len(delimiter)
					s = s[end+len(delimiter):]
					continue
				}
			}
			_, n := utf8.DecodeRuneInString(s)
			literal(s[:n])
			s = s[n:]
		}
	}
	render(source, limit, false)
	// Only the fixed tags above bypass html/template; all source text is escaped.
	return template.HTML(out.String())
}

func quoteDelimiter(s, delimiter string) int {
	for i := len(delimiter); i < len(s); {
		if strings.HasPrefix(s[i:], delimiter) {
			end := i + len(delimiter)
			if (i == 0 || s[i-1] != delimiter[0]) && (end == len(s) || s[end] != delimiter[0]) {
				return i
			}
		}
		// Bold markers inside a code span cannot close a bold span.
		if delimiter != "`" && s[i] == '`' {
			if end := quoteDelimiter(s[i:], "`"); end > 1 {
				i += end + 1
				continue
			}
		}
		i++
	}
	return -1
}
