package meta

import "testing"

func TestParseLimits(t *testing.T) {
	cases := []struct {
		name    string
		params  map[string]string
		want    Limits
		wantErr bool
	}{
		{
			name:   "empty",
			params: map[string]string{},
			want:   Limits{},
		},
		{
			name: "plain integers",
			params: map[string]string{
				KeyRBPS:  "104857600",
				KeyWBPS:  "52428800",
				KeyRIOPS: "400",
				KeyWIOPS: "200",
			},
			want: Limits{RBPS: 104857600, WBPS: 52428800, RIOPS: 400, WIOPS: 200},
		},
		{
			name:   "kubernetes quantities",
			params: map[string]string{KeyRBPS: "40Mi", KeyWBPS: "20Mi"},
			want:   Limits{RBPS: 40 * 1024 * 1024, WBPS: 20 * 1024 * 1024},
		},
		{
			name:   "device key is ignored",
			params: map[string]string{KeyDevice: "/dev/drbd1000", KeyWIOPS: "500"},
			want:   Limits{WIOPS: 500},
		},
		{
			name:    "unparseable value",
			params:  map[string]string{KeyRBPS: "not-a-number"},
			wantErr: true,
		},
		{
			name:    "negative value",
			params:  map[string]string{KeyRIOPS: "-5"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLimits(tc.params)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil (result %+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
