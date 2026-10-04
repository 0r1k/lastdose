package art

import "testing"

func checkASCII(t *testing.T, name string, lines []string) {
	t.Helper()
	for _, l := range lines {
		for _, r := range l {
			if r < 0x20 || r > 0x7e {
				t.Fatalf("%s: non-ASCII %q in %q", name, r, l)
			}
		}
	}
}

func TestFramesStable(t *testing.T) {
	// Every frame must be the same size so animation does not jump around.
	for f := 0; f < len(idleSeq)+len(partySeq)+len(sleepSeq); f++ {
		for hat := -1; hat < HatCount; hat++ {
			for name, fr := range map[string][]string{
				"idle": Owl(f, hat), "party": Party(f, hat), "sleep": Sleeping(f),
			} {
				checkASCII(t, name, fr)
				if len(fr) != Height {
					t.Fatalf("%s frame %d has %d lines", name, f, len(fr))
				}
				for _, l := range fr {
					if len(l) != Width {
						t.Fatalf("%s frame %d hat %d: width %d: %q", name, f, hat, len(l), l)
					}
				}
			}
		}
	}
}

func TestBadgeShape(t *testing.T) {
	for frame := 0; frame < 10; frame++ {
		for _, lit := range []bool{true, false} {
			b := Badge(Party(frame, 3), "Forged in Silence", "8 DAYS", "Discipline is quiet. Its results are loud.", frame, lit, Styles{})
			checkASCII(t, "badge", b)
			for _, l := range b {
				if len(l) != BadgeInner+2 {
					t.Fatalf("badge line width %d: %q", len(l), l)
				}
			}
		}
	}
}
