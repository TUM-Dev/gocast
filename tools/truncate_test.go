package tools

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
)

// TruncateHtml is a hand-rolled parser fed lecture and course descriptions, which are
// user-supplied. Every case here pins current behaviour; the ones marked BUG document
// output that is wrong but shipped, so a fix is a deliberate, visible change.
func TestTruncateHtml(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		maxlen   int
		ellipsis string
		want     string
		wantErr  error
	}{
		{
			name:     "an empty input yields an empty result and no ellipsis",
			in:       "",
			maxlen:   10,
			ellipsis: "...",
			want:     "",
		},
		{
			// A zero budget is the one short-circuit the function takes before parsing,
			// so it is the only way to get no ellipsis at all.
			name:     "a maxlen of zero yields an empty result and no ellipsis",
			in:       "hello world",
			maxlen:   0,
			ellipsis: "...",
			want:     "",
		},
		{
			// BUG: input that fits should come back unchanged. The ellipsis is appended
			// unconditionally, so a short description gains a "..." that promises text
			// which does not exist.
			name:     "input shorter than maxlen still gains an ellipsis",
			in:       "hello",
			maxlen:   10,
			ellipsis: "...",
			want:     "hello...",
		},
		{
			name:     "input exactly maxlen still gains an ellipsis",
			in:       "hello",
			maxlen:   5,
			ellipsis: "...",
			want:     "hello...",
		},
		{
			name:     "input longer than maxlen is cut at the visible-character budget",
			in:       "hello world",
			maxlen:   5,
			ellipsis: "...",
			want:     "hello...",
		},
		{
			name:     "an empty ellipsis appends nothing",
			in:       "hello world",
			maxlen:   5,
			ellipsis: "",
			want:     "hello",
		},
		{
			// A negative budget fails the loop guard on the first iteration, so nothing
			// is scanned and exactly one rune survives. Pinned because it is surprising:
			// a caller passing a computed negative length gets a one-character string,
			// not an empty one.
			name:     "a negative maxlen emits the first rune plus the ellipsis",
			in:       "hello",
			maxlen:   -3,
			ellipsis: "...",
			want:     "h...",
		},
		{
			name:     "markup does not consume the visible budget",
			in:       "<b>hello</b>",
			maxlen:   5,
			ellipsis: "...",
			want:     "<b>hello...</b>",
		},
		{
			// The ellipsis goes inside the reopened tags, not after them. Worth pinning
			// either way: it decides whether the "..." inherits the tag's styling.
			name:     "the ellipsis is placed inside the still-open tags",
			in:       "<b>hello</b>",
			maxlen:   3,
			ellipsis: "...",
			want:     "<b>hel...</b>",
		},
		{
			name:     "nested open tags are closed in reverse order",
			in:       "<div><p><b>abcdefgh</b></p></div>",
			maxlen:   4,
			ellipsis: "...",
			want:     "<div><p><b>abcd...</b></p></div>",
		},
		{
			name:     "tags closed before the cut are not reopened",
			in:       "<b>a</b><i>b</i>cdef",
			maxlen:   3,
			ellipsis: "...",
			want:     "<b>a</b><i>b</i>c...",
		},
		{
			name:     "an unclosed tag at the end of the input is closed for us",
			in:       "<b>unclosed",
			maxlen:   3,
			ellipsis: "...",
			want:     "<b>unc...</b>",
		},
		{
			// A </br> or </img> in the output is invalid HTML and confuses browsers, so
			// void elements must never reach the close stack.
			name:     "a br is not pushed on the close stack",
			in:       "a<br>bcdefg",
			maxlen:   4,
			ellipsis: "...",
			want:     "a<br>bcd...",
		},
		{
			name:     "an img with attributes is not pushed on the close stack",
			in:       `a<img src="x.png" alt="y">bcdefg`,
			maxlen:   4,
			ellipsis: "...",
			want:     `a<img src="x.png" alt="y">bcd...`,
		},
		{
			name:     "a self-closed hr is not pushed on the close stack",
			in:       "a<hr/>bcdefg",
			maxlen:   4,
			ellipsis: "...",
			want:     "a<hr/>bcd...",
		},
		{
			// BUG: the void-element list is matched case-sensitively, so uppercase
			// markup (common in pasted rich-text) produces the invalid "</BR>".
			name:     "an uppercase BR is wrongly closed",
			in:       "<BR>abcdef",
			maxlen:   3,
			ellipsis: "...",
			want:     "<BR>abc...</BR>",
		},
		{
			name:     "tag attributes do not consume the visible budget",
			in:       `<p class="lead" data-x="1">abcdef</p>`,
			maxlen:   3,
			ellipsis: "...",
			want:     `<p class="lead" data-x="1">abc...</p>`,
		},
		{
			// A half-written entity ("&am") renders as literal garbage, so the cut must
			// land on a whole entity and count it as the single character it displays.
			name:     "a named entity counts as one visible character",
			in:       "ab&amp;cd",
			maxlen:   3,
			ellipsis: "...",
			want:     "ab&amp;...",
		},
		{
			name:     "an entity is never cut in half at the budget boundary",
			in:       "a&amp;bcdefg",
			maxlen:   2,
			ellipsis: "...",
			want:     "a&amp;...",
		},
		{
			name:     "a numeric entity counts as one visible character",
			in:       "&#8212;abcdef",
			maxlen:   2,
			ellipsis: "...",
			want:     "&#8212;a...",
		},
		{
			name:     "consecutive entities each count once",
			in:       "&amp;&amp;&amp;",
			maxlen:   2,
			ellipsis: "...",
			want:     "&amp;&amp;...",
		},
		{
			// A bare ampersand is not an entity; it must still count as one character
			// rather than swallowing the text behind it.
			name:     "a bare ampersand counts as one visible character",
			in:       "a&nope b c d",
			maxlen:   3,
			ellipsis: "...",
			want:     "a&n...",
		},
		{
			// German lecture titles are the common case here; a cut inside a rune
			// produces a replacement character in the browser.
			name:     "umlauts are counted and cut per rune",
			in:       "äöüßabcdef",
			maxlen:   3,
			ellipsis: "...",
			want:     "äöü...",
		},
		{
			name:     "a four-byte emoji is never split",
			in:       "😀😀😀abc",
			maxlen:   2,
			ellipsis: "...",
			want:     "😀😀...",
		},
		{
			name:     "CJK characters count one each",
			in:       "中文字符测试",
			maxlen:   3,
			ellipsis: "...",
			want:     "中文字...",
		},
		{
			name:     "a multi-byte rune inside a tag is cut and the tag reclosed",
			in:       "<b>ä</b>öü",
			maxlen:   1,
			ellipsis: "...",
			want:     "<b>ä...</b>",
		},
		{
			// Whitespace is deliberately excluded from the budget, so a spaced-out
			// string yields more characters than maxlen suggests. Pinned because it is
			// the single most surprising property of the function.
			name:     "whitespace does not consume the visible budget",
			in:       "a b c d e f",
			maxlen:   3,
			ellipsis: "...",
			want:     "a b c...",
		},
		{
			name:     "leading whitespace is preserved and uncounted",
			in:       "   abcdef",
			maxlen:   2,
			ellipsis: "...",
			want:     "   ab...",
		},
		{
			name:     "a closing tag with no opener is rejected",
			in:       "abc</b>defgh",
			maxlen:   4,
			ellipsis: "...",
			wantErr:  ErrUnbalancedTags,
		},
		{
			name:     "a closing tag that does not match the top of the stack is rejected",
			in:       "<b>abc</i>defgh",
			maxlen:   4,
			ellipsis: "...",
			wantErr:  ErrUnbalancedTags,
		},
		{
			name:     "input that is only a closing tag is rejected",
			in:       "</b>",
			maxlen:   3,
			ellipsis: "...",
			wantErr:  ErrUnbalancedTags,
		},
		{
			// A trailing "<" never reaches the tag parser because the scan stops at the
			// end of the buffer first; contrast with TestTruncateHtmlStrayLessThanPanics.
			name:     "a trailing stray less-than is copied through",
			in:       "<",
			maxlen:   3,
			ellipsis: "...",
			want:     "<...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TruncateHtml([]byte(tt.in), tt.maxlen, tt.ellipsis)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("TruncateHtml(%q, %d) error = %v, want %v", tt.in, tt.maxlen, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != nil {
					t.Errorf("TruncateHtml(%q, %d) = %q, want nil output alongside the error", tt.in, tt.maxlen, got)
				}
				return
			}
			if string(got) != tt.want {
				t.Errorf("TruncateHtml(%q, %d) = %q, want %q", tt.in, tt.maxlen, got, tt.want)
			}
			if !utf8.Valid(got) {
				t.Errorf("TruncateHtml(%q, %d) produced invalid UTF-8: %q", tt.in, tt.maxlen, got)
			}
		})
	}
}

// BUG: a "<" that does not start a tag makes TagExpr.FindSubmatch return nil, which is
// then indexed unguarded. "5 < 10" or an HTML comment in a course description crashes
// the request. Pinned as a panic so a fix turns this test red rather than passing
// silently.
func TestTruncateHtmlStrayLessThanPanics(t *testing.T) {
	inputs := []string{
		"a < b and more text",
		"5 < 10 and 3 > 1 blah",
		"<!-- a comment -->abcdef",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("TruncateHtml(%q) no longer panics; the nil-match crash appears fixed, update this test", in)
				}
			}()
			_, _ = TruncateHtml([]byte(in), 5, "...")
		})
	}
}

// BUG: the end-of-buffer check compares bufPtr against len(buf)-1, which only holds
// when the final rune is one byte wide. An input whose last rune is multi-byte and
// whose budget is not exhausted before it falls through to the tag parser with no tag
// left to match, and crashes. Only the final rune decides, so "Grüße" is safe and "bä"
// is not -- which is how this survived unnoticed in a German university's codebase.
func TestTruncateHtmlPanicsOnTrailingMultiByteRune(t *testing.T) {
	inputs := []string{"ä", "bä", "ö", "😀", "中", "Vorlesung über Prüfungsordnungä"}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("TruncateHtml(%q) no longer panics; the end-of-buffer check appears fixed, update this test", in)
				}
			}()
			_, _ = TruncateHtml([]byte(in), 100, "...")
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		length int
		want   string
	}{
		{
			name:   "an empty input yields an empty string",
			in:     "",
			length: 5,
			want:   "",
		},
		{
			name:   "a zero length yields an empty string",
			in:     "hello",
			length: 0,
			want:   "",
		},
		{
			// Same unconditional-ellipsis bug as TruncateHtml, surfaced through the
			// wrapper that templates actually call.
			name:   "input shorter than the length still gains an ellipsis",
			in:     "hello",
			length: 10,
			want:   "hello...",
		},
		{
			name:   "input exactly the length still gains an ellipsis",
			in:     "hello",
			length: 5,
			want:   "hello...",
		},
		{
			name:   "input longer than the length is truncated",
			in:     "hello world",
			length: 5,
			want:   "hello...",
		},
		{
			// Counting is per rune, not per byte: "äö" is four bytes but two characters,
			// and neither is split.
			name:   "multi-byte input is cut by rune, not by byte",
			in:     "äöü",
			length: 2,
			want:   "äö...",
		},
		{
			name:   "emoji are cut by rune",
			in:     "😀😀😀",
			length: 2,
			want:   "😀😀...",
		},
		{
			// BUG: the error branch assigns to a blank identifier and falls through, so
			// unbalanced markup silently discards the whole text instead of degrading to
			// something readable.
			name:   "unbalanced markup silently yields an empty string",
			in:     "a</b>",
			length: 3,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.in, tt.length); got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.length, got, tt.want)
			}
		})
	}
}

// Randomised smoke test over markup-shaped input. It does not recompute the visible
// count (that would just reimplement the function); it pins the three properties that
// must hold for any input: no crash, valid UTF-8 out, and output that is the input
// prefix plus the ellipsis and closing tags — never unrelated bytes.
func TestTruncateHtmlPropertiesOnGeneratedInput(t *testing.T) {
	tokens := []string{
		"a", "b", " ", "  ", "ä", "ö", "ß", "😀", "中", "&amp;", "&#8212;", "&",
		"<b>", "</b>", "<i>", "</i>", "<br>", "<hr/>", `<img src="x">`, "\n", "\t",
	}
	const ellipsis = "…"

	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		var sb strings.Builder
		for n := rng.Intn(12); n >= 0; n-- {
			sb.WriteString(tokens[rng.Intn(len(tokens))])
		}
		// Every input is terminated with an ASCII byte: an input ending in a multi-byte
		// rune crashes (see TestTruncateHtmlPanicsOnTrailingMultiByteRune), which would
		// otherwise mask every other property this loop checks.
		sb.WriteString("z")
		in := sb.String()
		maxlen := rng.Intn(12)

		got, err := TruncateHtml([]byte(in), maxlen, ellipsis)
		if err != nil {
			if !errors.Is(err, ErrUnbalancedTags) {
				t.Fatalf("TruncateHtml(%q, %d) returned unexpected error %v", in, maxlen, err)
			}
			continue
		}
		if !utf8.Valid(got) {
			t.Fatalf("TruncateHtml(%q, %d) produced invalid UTF-8: %q", in, maxlen, got)
		}
		out := string(got)
		if out == "" {
			continue
		}
		cut := strings.Index(out, ellipsis)
		if cut < 0 {
			t.Fatalf("TruncateHtml(%q, %d) = %q, dropped the ellipsis", in, maxlen, out)
		}
		if !strings.HasPrefix(in, out[:cut]) {
			t.Fatalf("TruncateHtml(%q, %d) = %q, kept text that is not a prefix of the input", in, maxlen, out)
		}
		for _, rest := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(out[cut:], ellipsis), ">"), ">") {
			if rest != "" && !strings.HasPrefix(rest, "</") {
				t.Fatalf("TruncateHtml(%q, %d) = %q, appended something other than closing tags", in, maxlen, out)
			}
		}
	}
}
