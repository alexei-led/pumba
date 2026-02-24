package netem

import (
	"testing"
	"time"

	"github.com/alexei-led/pumba/pkg/chaos"
	"github.com/alexei-led/pumba/pkg/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validationFixtures(t *testing.T) (*container.MockClient, *chaos.GlobalParams, *Params) {
	return container.NewMockClient(t),
		&chaos.GlobalParams{Names: []string{"test"}},
		&Params{Iface: "eth0", Duration: time.Second}
}

func TestNewCorruptCommand_Validation(t *testing.T) {
	tests := []struct {
		name        string
		percent     float64
		correlation float64
		wantErr     string
	}{
		{"valid", 50.0, 25.0, ""},
		{"negative percent", -1.0, 0, "invalid corrupt percent"},
		{"percent over 100", 101.0, 0, "invalid corrupt percent"},
		{"negative correlation", 50.0, -1.0, "invalid corrupt correlation"},
		{"correlation over 100", 50.0, 101.0, "invalid corrupt correlation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, gParams, nParams := validationFixtures(t)
			cmd, err := NewCorruptCommand(client, gParams, nParams, tt.percent, tt.correlation)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cmd)
			}
		})
	}
}

func TestNewLossCommand_Validation(t *testing.T) {
	tests := []struct {
		name        string
		percent     float64
		correlation float64
		wantErr     string
	}{
		{"valid", 30.0, 10.0, ""},
		{"negative percent", -1.0, 0, "invalid loss percent"},
		{"percent over 100", 101.0, 0, "invalid loss percent"},
		{"negative correlation", 30.0, -1.0, "invalid loss correlation"},
		{"correlation over 100", 30.0, 101.0, "invalid loss correlation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, gParams, nParams := validationFixtures(t)
			cmd, err := NewLossCommand(client, gParams, nParams, tt.percent, tt.correlation)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cmd)
			}
		})
	}
}

func TestNewDuplicateCommand_Validation(t *testing.T) {
	tests := []struct {
		name        string
		percent     float64
		correlation float64
		wantErr     string
	}{
		{"valid", 10.0, 5.0, ""},
		{"negative percent", -1.0, 0, "invalid duplicate percent"},
		{"percent over 100", 101.0, 0, "invalid duplicate percent"},
		{"negative correlation", 10.0, -1.0, "invalid duplicate correlation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, gParams, nParams := validationFixtures(t)
			cmd, err := NewDuplicateCommand(client, gParams, nParams, tt.percent, tt.correlation)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cmd)
			}
		})
	}
}

func TestNewRateCommand_Validation(t *testing.T) {
	tests := []struct {
		name     string
		rate     string
		cellSize int
		wantErr  string
	}{
		{"valid kbit", "100kbit", 0, ""},
		{"valid mbit", "10mbit", 0, ""},
		{"valid gbit", "1gbit", 0, ""},
		{"empty rate", "", 0, "undefined rate limit"},
		{"invalid rate", "notarate", 0, "invalid rate"},
		{"invalid cell size", "100mbit", -1, "invalid cell size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, gParams, nParams := validationFixtures(t)
			cmd, err := NewRateCommand(client, gParams, nParams, tt.rate, 0, tt.cellSize, 0)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cmd)
			}
		})
	}
}

func TestNewLossGECommand_Validation(t *testing.T) {
	tests := []struct {
		name               string
		pg, pb, oneH, oneK float64
		wantErr            string
	}{
		{"valid", 50, 50, 50, 50, ""},
		{"invalid pg", -1, 50, 50, 50, "invalid pg"},
		{"invalid pb", 50, 101, 50, 50, "invalid pb"},
		{"invalid oneH", 50, 50, -1, 50, "invalid loss probability"},
		{"invalid oneK", 50, 50, 50, 101, "invalid loss probability"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, gParams, nParams := validationFixtures(t)
			cmd, err := NewLossGECommand(client, gParams, nParams, tt.pg, tt.pb, tt.oneH, tt.oneK)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cmd)
			}
		})
	}
}

func TestNewLossStateCommand_Validation(t *testing.T) {
	tests := []struct {
		name                    string
		p13, p31, p32, p23, p14 float64
		wantErr                 string
	}{
		{"valid", 50, 50, 50, 50, 50, ""},
		{"invalid p13", -1, 50, 50, 50, 50, "invalid p13"},
		{"invalid p31", 50, 101, 50, 50, 50, "invalid p31"},
		{"invalid p32", 50, 50, -1, 50, 50, "invalid p32"},
		{"invalid p23", 50, 50, 50, 101, 50, "invalid p23"},
		{"invalid p14", 50, 50, 50, 50, 101, "invalid p14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, gParams, nParams := validationFixtures(t)
			cmd, err := NewLossStateCommand(client, gParams, nParams, tt.p13, tt.p31, tt.p32, tt.p23, tt.p14)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cmd)
			}
		})
	}
}
