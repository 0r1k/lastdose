package achievements

import (
	"testing"
	"time"
)

func TestLadderDoublesAndCoversFiveYears(t *testing.T) {
	if All[0].Threshold != 24*time.Hour {
		t.Fatalf("first = %v, want 24h", All[0].Threshold)
	}
	for i := 1; i < len(All); i++ {
		if All[i].Threshold != 2*All[i-1].Threshold {
			t.Fatalf("step %d = %v, want double of %v", i, All[i].Threshold, All[i-1].Threshold)
		}
	}
	fiveYears := time.Duration(5*365.25*24) * time.Hour
	if last := All[len(All)-1].Threshold; last < fiveYears {
		t.Fatalf("last = %v, does not cover 5 years", last)
	}
}

func TestUnlockedAndProgress(t *testing.T) {
	cases := []struct {
		elapsed time.Duration
		want    int
	}{
		{0, 0},
		{24*time.Hour - time.Second, 0},
		{24 * time.Hour, 1},
		{96 * time.Hour, 3},
		{100000 * time.Hour, len(All)},
	}
	for _, c := range cases {
		if got := Unlocked(c.elapsed); got != c.want {
			t.Errorf("Unlocked(%v) = %d, want %d", c.elapsed, got, c.want)
		}
	}
	if p := Progress(36 * time.Hour); p != 0.5 {
		t.Errorf("Progress(36h) = %v, want 0.5", p)
	}
	if _, ok := Next(100000 * time.Hour); ok {
		t.Error("Next after everything unlocked should be false")
	}
}
