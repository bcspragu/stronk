package fto

import "testing"

func TestWeightString(t *testing.T) {
	tests := []struct {
		desc string
		in   Weight
		want string
	}{
		{
			desc: "invalid unit",
			in:   Weight{Value: 123, Unit: "NOT_A_UNIT"},
			want: "UNKNOWN_UNIT",
		},
		{
			desc: "whole number",
			in:   Weight{Value: 1000, Unit: DeciPounds},
			want: "100",
		},
		{
			desc: "Fractional",
			in:   Weight{Value: 1005, Unit: DeciPounds},
			want: "100.5",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := test.in.String()
			if got != test.want {
				t.Errorf("String(%d, %q) = %q, want %q", test.in.Value, test.in.Unit, got, test.want)
			}
		})
	}
}
