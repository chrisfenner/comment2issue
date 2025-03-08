package main

import (
	"flag"
	"fmt"
	"iter"
	"maps"
	"os"
	"regexp"
	"sort"
	"strings"

	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func main() {
	flag.Parse()
	if err := mainErr(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func mainErr() error {
	if len(flag.Args()) == 0 {
		return fmt.Errorf("please provide at least one pdf to scrape comments from")
	}
	for _, filename := range flag.Args() {
		if err := scrapeFile(filename); err != nil {
			return err
		}
	}
	return nil
}

// Helper for collecting the result of an iterator
func collect[V any](i iter.Seq[V]) []V {
	var vs []V
	for v := range i {
		vs = append(vs, v)
	}
	return vs
}

// Helper for collecting and sorting the result of an iterator
func sorted[V interface{ ~int }](i iter.Seq[V]) []V {
	result := collect(i)
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func getCommentContents(annotation model.AnnotationRenderer) string {
	switch annotation.Type() {
	// Ignore these types of annotations, they aren't comments.
	case model.AnnLink, model.AnnPopup:
		return ""
		// The main types of comments we expect.
	case model.AnnHighLight:
		// TODO: pdfcpu doesn't provide the author info here, otherwise it'd be great to include.
		return annotation.ContentString()
	default:
		return ""
	}

}

func summarizeComment(c model.AnnotationRenderer) (string, string) {
	rawContents := getCommentContents(c)
	rawContents = strings.TrimSpace(rawContents)
	rawContents = strings.ReplaceAll(rawContents, "\r", "\n")

	// The comment may follow the scheme "[MINOR] Anticpated blah blah."
	r := regexp.MustCompile(`^\[(\w+)\] (.*)$`)
	matches := r.FindStringSubmatch(rawContents)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	// Unknown comment level.
	return "", rawContents
}

func markdownizeComment(page int, level string, content string) string {
	var result strings.Builder
	// The first line is the checkbox syntax and the page number.
	// TODO: author name here once we can find this info from pdfcpu or another PDF library.
	if level != "" {
		fmt.Fprintf(&result, "- [ ] Page %d (%v)\n", page, level)
	} else {
		fmt.Fprintf(&result, "- [ ] Page %d\n", page)
	}
	// The rest of the comment should be a block quote containing the comment.
	for _, line := range strings.Split(content, "\n") {
		fmt.Fprintf(&result, "    > %v\n", line)
	}
	return result.String()
}

func scrapeFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	cfg := model.NewDefaultConfiguration()
	annotations, err := pdfcpu.Annotations(f, []string{"1-"}, cfg)
	if err != nil {
		return err
	}

	pages := sorted(maps.Keys(annotations))
	for _, page := range pages {
		pageAnnotations := annotations[page]
		kinds := sorted(maps.Keys(pageAnnotations))
		for _, kind := range kinds {
			commentIDs := sorted(maps.Keys(pageAnnotations[kind].Map))
			for _, commentID := range commentIDs {
				level, comment := summarizeComment(pageAnnotations[kind].Map[commentID])
				if comment != "" {
					fmt.Print(markdownizeComment(page, level, comment))
				}
			}
		}
	}

	return nil
}
