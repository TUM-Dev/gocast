package tools

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrUnbalancedTags = errors.New("unbalanced tags")
	TagExpr           = regexp.MustCompile("<(/?)([A-Za-z0-9]+).*?>")
	EntityExpr        = regexp.MustCompile("&#?[A-Za-z0-9]+;")
)

// TruncateHtml will truncate a given byte slice to a maximum of maxlen visible
// characters and optionally append ellipsis. HTML tags are automatically closed
// generating valid truncated HTML.
// Based on: https://github.com/mborgerson/GoTruncateHtml
func TruncateHtml(buf []byte, maxlen int, ellipsis string) ([]byte, error) {
	// Here's the gist: Scan the input bytestream. While scanning, count the
	// number of visible characters--that is, characters which are not part of
	// markup tags. When a start tag is encountered, push the tag name onto a
	// stack. When visible character count >= maxlen, or the EOF is reached,
	// stop counting. Copy from the input stream the bytes from the start to the
	// current scanning pointer. Finally, pop each tag off the tag stack and
	// append it to the output stream in the form of a closing tag.

	// We will consider HTML or XHTML as valid input. The following elements,
	// called "Void Elements" need not conform to the XHTML <tag /> convention
	// of void elements and may appear simply as <tag>. Hence, if one of the
	// following is picked up by the tag expression as a start tag, do not add
	// it to the stack of tags that should be closed.
	voidElementTags := []string{
		"area", "base", "br", "col", "embed", "hr",
		"img", "input", "keygen", "link", "meta",
		"param", "source", "track", "wbr",
	}

	// Check to see if no input was provided.
	if len(buf) == 0 || maxlen == 0 {
		return []byte{}, nil
	}

	var tagStack []string
	visible := 0
	bufPtr := 0

	for bufPtr < len(buf) && visible < maxlen {

		// Move to nearest tag and count visible characters along the way.
		offset := 0
		visibleCharacterMaxReached := false
		entityDetected := false

	outer:
		for localOffset, runeValue := range string(buf[bufPtr:]) {
			offset = localOffset

			switch {
			case runeValue == '<':
				// Start of tag.
				break outer
			case runeValue == '&':
				// Possible start of HTML Entity
				loc := EntityExpr.FindIndex(buf[bufPtr+localOffset:])
				if loc != nil && loc[0] == 0 {
					// Entity found!
					entityDetected = true
					offset += loc[1] - 1 // Now pointing to ;
				}
				visible++
			case unicode.IsPrint(runeValue) && !unicode.IsSpace(runeValue):
				// Printable, non-space character. Increment visible count.
				visible++
			}

			// Check if the limit of visible characters has been reached.
			if visible >= maxlen {
				visibleCharacterMaxReached = true
				break
			}

			if entityDetected {
				break
			}
		}

		// Increment bufPtr to end of scanned section
		bufPtr += offset

		// Stop scanning if the end of the buffer was reached or if the max
		// desired visible characters was reached. bufPtr is a byte offset pointing
		// at the *start* of a rune, so the end of the buffer is one whole rune --
		// not one byte -- past it.
		_, lastRuneSize := utf8.DecodeRune(buf[bufPtr:])
		if visibleCharacterMaxReached || bufPtr+lastRuneSize >= len(buf) {
			break
		}

		// If an entity was detected, continue scanning for next tag
		if entityDetected {
			// Advance past the ;
			bufPtr++
			continue
		}

		// Now find the expression sub-matches
		matches := TagExpr.FindSubmatch(buf[bufPtr:])
		if matches == nil || !bytes.HasPrefix(buf[bufPtr:], matches[0]) {
			// TagExpr is unanchored, so a match that does not start at bufPtr belongs
			// to some later tag. Either way this '<' does not open a tag -- it is
			// literal text ("5 < 10") or a comment. Count it as a visible character
			// and keep scanning past it.
			visible++
			if visible >= maxlen {
				break
			}
			bufPtr++
			continue
		}
		tagName := string(matches[2])

		// Advance pointer to the end of the tag
		bufPtr += len(matches[0])

		// If this is a void element, do not count it as a start tag
		isVoidElement := false
		for _, voidElementTagName := range voidElementTags {
			if strings.EqualFold(tagName, voidElementTagName) {
				isVoidElement = true
				break
			}
		}
		if isVoidElement {
			continue
		}

		isStartTag := len(matches[1]) == 0

		if isStartTag {
			// This is a start tag. Push the tag to the stack.
			tagStack = append(tagStack, tagName)
		} else {
			// This is an end tag. First, check to make sure the end tag is
			// matches what's on top of the stack.
			// HTML tag names are case-insensitive, so <B>...</b> is balanced.
			if len(tagStack) == 0 || !strings.EqualFold(tagStack[len(tagStack)-1], tagName) {
				return nil, ErrUnbalancedTags
			}

			// Now, pop the tag stack.
			tagStack = tagStack[0 : len(tagStack)-1]
		}
	}

	// At this point, bufPtr points to the last rune that should be copied to
	// the output stream. Increment bufPtr past this rune, turning bufPtr into
	// the number of bytes that should be copied.
	_, size := utf8.DecodeRune(buf[bufPtr:])
	bufPtr += size

	// Copy the desired input to the output buffer.
	output := buf[0:bufPtr]

	// Copy the ellipsis, but only if something was actually cut. The scan stops
	// either because the visible budget ran out or because the input ended, and at
	// the boundary both happen at once -- so ask the only question that matters:
	// is there any input left beyond what we copied?
	if bufPtr < len(buf) {
		output = append(output, []byte(ellipsis)...)
	}

	// Finally, create a closing tag for each tag in the stack.
	for i := len(tagStack) - 1; i >= 0; i-- {
		output = append(output, []byte(fmt.Sprintf("</%s>", tagStack[i]))...)
	}

	return output, nil
}

// Truncate strips an input that can contain html to length.
func Truncate(input string, length int) string {
	tr, err := TruncateHtml([]byte(input), length, "...")
	if err != nil {
		// The markup is unbalanced, so no valid truncated HTML can be produced from
		// it. The text itself is still worth showing: drop the markup and truncate
		// what is left as plain text rather than blanking the whole description.
		tr, err = TruncateHtml(TagExpr.ReplaceAll([]byte(input), nil), length, "...")
		if err != nil {
			return ""
		}
	}
	return string(tr)
}
