package markdown

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

var (
	imgSrcPattern = regexp.MustCompile(`<img src="([^"]+)"`)
	aHrefPattern  = regexp.MustCompile(`<a href="([^"]+)"`)
)

func Render(mdText string) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.Strikethrough | parser.Footnotes | parser.Tables
	p := parser.NewWithExtensions(extensions)
	opts := html.RendererOptions{
		Flags: html.CommonFlags | html.HrefTargetBlank | html.LazyLoadImages,
	}
	renderer := html.NewRenderer(opts)
	htmlBytes := markdown.ToHTML([]byte(mdText), p, renderer)
	htmlStr := string(htmlBytes)
	htmlStr = imgSrcPattern.ReplaceAllString(htmlStr, `<img src="$1" loading="lazy"`)
	htmlStr = aHrefPattern.ReplaceAllStringFunc(htmlStr, addNoopener)
	return htmlStr
}

func addNoopener(match string) string {
	return match[:len(match)-1] + ` rel="noopener"`
}

func Excerpt(mdText string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 200
	}
	p := parser.NewWithExtensions(parser.CommonExtensions)
	htmlBytes := markdown.ToHTML([]byte(mdText), p, html.NewRenderer(html.RendererOptions{Flags: html.SkipHTML}))
	plain := stripTags(string(htmlBytes))
	if len([]rune(plain)) <= maxLen {
		return plain
	}
	runes := []rune(plain)
	return string(runes[:maxLen]) + "…"
}

func stripTags(s string) string {
	var buf bytes.Buffer
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				buf.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(buf.String())
}
