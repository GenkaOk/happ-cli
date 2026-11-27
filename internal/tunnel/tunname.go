package tunnel

import "strconv"

// utunNameRange is how many utun device numbers are probed when macOS needs a
// free TUN name.
const utunNameRange = 256

// pickUtunName returns the lowest free "utunN" name, or "" when every number in
// the range is taken. macOS only accepts utunN device names, while Linux uses
// descriptive interface names, so the default is platform-specific.
//
// taken reports whether a name is already in use.
func pickUtunName(taken func(string) bool) string {
	for n := 0; n < utunNameRange; n++ {
		name := "utun" + strconv.Itoa(n)
		if !taken(name) {
			return name
		}
	}
	return ""
}
