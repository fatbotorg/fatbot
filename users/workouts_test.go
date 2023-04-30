package users

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestIsOlderThan(t *testing.T) {
	var tests = []struct {
		workout Workout
		minuts  int
		want    bool
	}{
		{
			workout: Workout{
				Model: gorm.Model{
					CreatedAt: time.Now().Add(time.Duration(-15) * time.Minute),
				},
			},
			minuts: 30,
			want:   false,
		},
		{
			workout: Workout{
				Model: gorm.Model{
					CreatedAt: time.Now().Add(time.Duration(-31) * time.Minute),
				},
			},
			minuts: 30,
			want:   true,
		},
		{
			workout: Workout{
				Model: gorm.Model{
					CreatedAt: time.Now().Add(time.Duration(-5) * time.Minute),
				},
			},
			minuts: 5,
			want:   false,
		},
		{
			workout: Workout{
				Model: gorm.Model{
					CreatedAt: time.Now().Add(time.Duration(-5) * time.Minute),
				},
			},
			minuts: 30,
			want:   false,
		},
	}
	for _, tt := range tests {
		testAdminCmd := fmt.Sprintf("%s", tt.workout.CreatedAt)
		t.Run(testAdminCmd, func(t *testing.T) {
			ans := tt.workout.IsOlderThan(tt.minuts)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}
