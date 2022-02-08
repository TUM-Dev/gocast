package model

import (
	"testing"
)

func TestChat_SanitiseMessage(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			`hallo! schau doch mal hier: https://www.google.com ;)`,
			`hallo! schau doch mal hier: <a href="https://www.google.com" rel="nofollow noopener" target="_blank">https://www.google.com</a> ;)`,
		},
		{
			"Fiese nachricht:<script>alert('hello')</script>",
			"Fiese nachricht:&lt;script&gt;alert(&#39;hello&#39;)&lt;/script&gt;",
		},
		{
			"<img src='https://bad.org'>hehe</img>",
			"&lt;img src=&#39;<a href=\"https://bad.org&#39;&gt;hehe&lt;/img%3E\" rel=\"nofollow noopener\" target=\"_blank\">https://bad.org&amp;#39;&gt;hehe&lt;/img&amp;gt</a>;",
		},
		{
			`<a onblur="alert(secret)" href="http://www.google.com">Google</a>`,
			"&lt;a onblur=&#34;alert(secret)&#34; href=&#34;<a href=\"http://www.google.com&amp;quot;&gt;Google&lt;/a%3E\" rel=\"nofollow noopener\" target=\"_blank\">http://www.google.com&amp;#34;&gt;Google&lt;/a&amp;gt</a>;",
		},
		{
			"kann man de l'hospital nicht nur anwenden wenn 0/0 bzw unendlich/unendlich",
			"kann man de l&#39;hospital nicht nur anwenden wenn 0/0 bzw unendlich/unendlich",
		},
	}
	for _, testCase := range testCases {
		c := &Chat{Message: testCase.input}
		c.SanitiseMessage()
		// trim whitespace at end because blackfriday adds it
		if c.SanitizedMessage != testCase.expected {
			t.Errorf("Expected [%s], got [%s]", testCase.expected, c.SanitizedMessage)
		}
	}
}

func BenchmarkChat_SanitiseMessage(b *testing.B) {
	c := &Chat{}
	m := `hallo! schau doch mal hier: https://www.google.com ;)`
	for i := 0; i < b.N; i++ {
		c.Message = m
		c.SanitiseMessage()
	}
}
