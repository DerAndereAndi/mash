package cert

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
)

func TestExtractZoneTypeFromCert(t *testing.T) {
	tests := []struct {
		name    string
		cert    *x509.Certificate
		want    ZoneType
		wantErr bool
	}{
		{
			name: "GRID",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					OrganizationalUnit: []string{"GRID", "zone-123"},
				},
			},
			want: ZoneTypeGrid,
		},
		{
			name: "LOCAL",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					OrganizationalUnit: []string{"LOCAL", "zone-456"},
				},
			},
			want: ZoneTypeLocal,
		},
		{
			name: "TEST",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					OrganizationalUnit: []string{"TEST", "zone-789"},
				},
			},
			want: ZoneTypeTest,
		},
		{
			name: "empty OU",
			cert: &x509.Certificate{
				Subject: pkix.Name{},
			},
			wantErr: true,
		},
		{
			name:    "nil cert",
			cert:    nil,
			wantErr: true,
		},
		{
			name: "unknown zone type",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					OrganizationalUnit: []string{"UNKNOWN_TYPE"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractZoneTypeFromCert(tt.cert)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractZoneTypeFromCert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExtractZoneTypeFromCert() = %v, want %v", got, tt.want)
			}
		})
	}
}
