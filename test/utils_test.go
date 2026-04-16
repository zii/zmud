package test

import (
	"fmt"
	"strings"
	"testing"

	"zmud/lib"
)

func TestStripANSI(t *testing.T) {
	s := " orien is being assaulted for the second time."
	a, b, c, d := lib.StripANSI(s)
	t.Logf("%q, %q, %q, %q\n", a, b, c, d)
}

func TestCleanWrap(t *testing.T) {
	cases := []string{
		"/ \r\n\r\nThe Two Towers is running",
		"\r\n\x1b[0m\r\nYou have not yet set your email address.",
		"The Orc attacks\r\nwith a sword.",
		"At Level 10\r\nyou get a new skill.",
		"choose dunlending\r\n                     choose eorling\r\n",
	}
	for _, s := range cases {
		c := lib.CleanWrap(s)
		t.Logf("%q\n", c)
	}
}

func TestRemoveCr(t *testing.T) {
	text := "   _____     _____\r\n__|     |___|     |           ^^        .            .     .          \r\n                  |         _/  \\  .             .            ^   . | |_| |_| |\r\n                  |        /     \\           /\\        .     ^^^     \\       /\r\n                 /       _/       \\_  .     /^^|            /~~~\\_    |     |  \r\n                /       /           \\_     /^^^^\\__  .    /~ ^^^  \\   | |_| | \r\n               /      _/              \\  |~  _     \\    _/         \\_ |     | \r\n              /      /                 \\/   /_\\     \\  /~            \\|     |\r\n             |    __/                 _/  _/   _-\\   \\/               |     | \r\n __     __   |  _/                   /   /        \\  /                |     | \r\n|  |   |  |  | /   The              /   /          \\/              __/       \r\n|  |   |  |  |/      Two           /                             _/        \r\n|  |   |  |  |         Towers    _/       Celebrating           /          \r\n|  |   |  |  |                __/       31 Years Online!      ==|              \r\n|__|   |__|  |              _/                             ===__|            \r\n             |            _/                          =====  /                 \r\n             |           /            ======         =     _/       est. 1994          \r\n             |       ___/         ====      ==     ==     /                    \r\n             |    __/                         =====      / \r\r\n\nThe Two Towers is running the TMI-2 1.1.1 mudlib on MudOS v22pre8\r\r\n\nPlease enter the name 'new' if you are new to The Two Towers.\r\nYour name? "
	text = lib.RemoveCr(text)
	lines := strings.Split(text, "\n")
	fmt.Println(len(lines))
}
