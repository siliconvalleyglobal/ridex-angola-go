package zones

import "testing"

func TestValidateBounds(t *testing.T) {
	tests := []struct {
		name string
		args [4]float64
		want bool
	}{
		{name: "Luanda rectangle", args: [4]float64{-9.2, 13.1, -8.7, 13.6}, want: true},
		{name: "reversed latitude", args: [4]float64{-8.7, 13.1, -9.2, 13.6}, want: false},
		{name: "reversed longitude", args: [4]float64{-9.2, 13.6, -8.7, 13.1}, want: false},
		{name: "outside latitude", args: [4]float64{-91, 13.1, -8.7, 13.6}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateBounds(tt.args[0], tt.args[1], tt.args[2], tt.args[3]); got != tt.want {
				t.Fatalf("ValidateBounds() = %v, want %v", got, tt.want)
			}
		})
	}
}
