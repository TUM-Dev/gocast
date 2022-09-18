package realtime

import (
	"strings"
	"testing"
)

func TestChannelPathMatch(t *testing.T) {
	t.Run("Simple Case", func(t *testing.T) {
		simplePath := []string{"example", "path", "simple"}
		channel := Channel{
			path:        simplePath,
			handlers:    ChannelHandlers{},
			subscribers: ChannelSubscribers{},
		}

		pathString := strings.Join(simplePath, channelPathSep)
		if match, _ := channel.PathMatches(pathString); !match {
			t.Errorf("channel.PathMatches(%s) = (false, [...]), want (true, [...])", pathString)
		}

		pathString = pathString + "2"
		if match, _ := channel.PathMatches(pathString); match {
			t.Errorf("channel.PathMatches(%s) = (true, [...]), want (false, [...])", pathString)
		}
	})

	t.Run("With Params", func(t *testing.T) {
		variablePath := []string{"example", ":var1", "path", ":var2", ":var3"}
		var1 := "foo"
		var2 := "bar"
		var3 := "blah123"
		validPath := "example/" + var1 + "/path/" + var2 + "/" + var3
		invalidPath := "example/" + var1 + "/pathX/" + var2 + "/" + var3

		channel := Channel{
			path:        variablePath,
			handlers:    ChannelHandlers{},
			subscribers: ChannelSubscribers{},
		}

		if match, vars := channel.PathMatches(validPath); match {
			if vars["var1"] != var1 {
				t.Errorf("channel.PathMatches(%s) = (true, { \"var1\": \"%s\", [...] }), want (true, { \"var1\": \"%s\", [...] })", validPath, vars["var1"], var1)
			}
			if vars["var2"] != var2 {
				t.Errorf("channel.PathMatches(%s) = (true, { \"var2\": \"%s\", [...] }), want (true, { \"var2\": \"%s\", [...] })", validPath, vars["var2"], var2)
			}
			if vars["var3"] != var3 {
				t.Errorf("channel.PathMatches(%s) = (true, { \"var3\": \"%s\", [...] }), want (true, { \"var3\": \"%s\", [...] })", validPath, vars["var3"], var3)
			}
		} else {
			t.Errorf("channel.PathMatches(%s) = (false, [...]), want (true, [...])", validPath)
		}

		if match, _ := channel.PathMatches(invalidPath); match {
			t.Errorf("channel.PathMatches(%s) = (true, [...]), want (false, [...])", invalidPath)
		}
	})
}
